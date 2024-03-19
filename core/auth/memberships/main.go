package main

import (
	"database/sql"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/ncruces/go-sqlite3"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
)

var port = flag.Int("port", 8080, "ort to listen on")
var dbPath = flag.String("db-path", "memberships.db", "Path to SQLite file")

//go:embed index.html
var indexHTML string

//go:embed group.html
var groupHTML string

//go:embed static
var staticResources embed.FS

type Store interface {
	CreateGroup(owner string, group Group) error
	AddChildGroup(parent, child string) error
	GetGroupsOwnedBy(user string) ([]Group, error)
	GetMembershipGroups(user string) ([]Group, error)
	IsGroupOwner(user, group string) (bool, error)
	AddGroupMember(user, group string) error
	AddGroupOwner(user, group string) error
	GetGroupOwners(group string) ([]string, error)
	GetGroupMembers(group string) ([]string, error)
	GetGroupDescription(group string) (string, error)
	GetAvailableGroupsAsChild(group string) ([]string, error)
}

type Server struct {
	store Store
}

type Group struct {
	Name        string
	Description string
}

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS groups (
            name TEXT PRIMARY KEY,
            description TEXT
        );

        CREATE TABLE IF NOT EXISTS owners (
            username TEXT,
            group_name TEXT,
            FOREIGN KEY(group_name) REFERENCES groups(name)
        );

        CREATE TABLE IF NOT EXISTS group_to_group (
            parent_group TEXT,
            child_group TEXT,
            FOREIGN KEY(parent_group) REFERENCES groups(name),
            FOREIGN KEY(child_group) REFERENCES groups(name)
        );

        CREATE TABLE IF NOT EXISTS user_to_group (
            username TEXT,
            group_name TEXT,
            FOREIGN KEY(group_name) REFERENCES groups(name)
        );`)
	if err != nil {
		return nil, err
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) queryGroups(query string, args ...interface{}) ([]Group, error) {
	groups := make([]Group, 0)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var group Group
		if err := rows.Scan(&group.Name, &group.Description); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *SQLiteStore) GetGroupsOwnedBy(user string) ([]Group, error) {
	query := `
        SELECT groups.name, groups.description
        FROM groups
        JOIN owners ON groups.name = owners.group_name
        WHERE owners.username = ?`
	return s.queryGroups(query, user)
}

func (s *SQLiteStore) GetMembershipGroups(user string) ([]Group, error) {
	query := `
        SELECT groups.name, groups.description
        FROM groups
        JOIN user_to_group ON groups.name = user_to_group.group_name
        WHERE user_to_group.username = ?`
	return s.queryGroups(query, user)
}

func (s *SQLiteStore) CreateGroup(owner string, group Group) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	query := `INSERT INTO groups (name, description) VALUES (?, ?)`
	if _, err := tx.Exec(query, group.Name, group.Description); err != nil {
		sqliteErr, ok := err.(*sqlite3.Error)
		if ok && sqliteErr.ExtendedCode() == 1555 {
			return fmt.Errorf("Group with the name %s already exists", group.Name)
		}
		return err
	}
	query = `INSERT INTO owners (username, group_name) VALUES (?, ?)`
	if _, err := tx.Exec(query, owner, group.Name); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) IsGroupOwner(user, group string) (bool, error) {
	query := `
        SELECT EXISTS (
            SELECT 1
            FROM owners
            WHERE username = ? AND group_name = ?
        )`
	var exists bool
	if err := s.db.QueryRow(query, user, group).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SQLiteStore) userGroupPairExists(tx *sql.Tx, table, user, group string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE username = ? AND group_name = ?)", table)
	var exists bool
	if err := tx.QueryRow(query, user, group).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SQLiteStore) AddGroupMember(user, group string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	existsInUserToGroup, err := s.userGroupPairExists(tx, "user_to_group", user, group)
	if err != nil {
		return err
	}
	if existsInUserToGroup {
		return fmt.Errorf("%s is already a member of group %s", user, group)
	}
	if _, err := tx.Exec(`INSERT INTO user_to_group (username, group_name) VALUES (?, ?)`, user, group); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) AddGroupOwner(user, group string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	existsInOwners, err := s.userGroupPairExists(tx, "owners", user, group)
	if err != nil {
		return err
	}
	if existsInOwners {
		return fmt.Errorf("%s is already an owner of group %s", user, group)
	}
	if _, err = tx.Exec(`INSERT INTO owners (username, group_name) VALUES (?, ?)`, user, group); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) getUsersByGroup(table, group string) ([]string, error) {
	query := fmt.Sprintf("SELECT username FROM %s WHERE group_name = ?", table)
	rows, err := s.db.Query(query, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []string
	for rows.Next() {
		var username string
		if err := rows.Scan(&username); err != nil {
			return nil, err
		}
		users = append(users, username)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

func (s *SQLiteStore) GetGroupOwners(group string) ([]string, error) {
	return s.getUsersByGroup("owners", group)
}

func (s *SQLiteStore) GetGroupMembers(group string) ([]string, error) {
	return s.getUsersByGroup("user_to_group", group)
}

func (s *SQLiteStore) GetGroupDescription(group string) (string, error) {
	var description string
	query := `SELECT description FROM groups WHERE name = ?`
	if err := s.db.QueryRow(query, group).Scan(&description); err != nil {
		return "", err
	}
	return description, nil
}

func (s *SQLiteStore) parentChildGroupPairExists(tx *sql.Tx, parent, child string) (bool, error) {
	query := `SELECT EXISTS (SELECT 1 FROM group_to_group WHERE parent_group = ? AND child_group = ?)`
	var exists bool
	if err := tx.QueryRow(query, parent, child).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SQLiteStore) AddChildGroup(parent, child string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	existsInGroupToGroup, err := s.parentChildGroupPairExists(tx, parent, child)
	if err != nil {
		return err
	}
	if existsInGroupToGroup {
		return fmt.Errorf("child group name %s already exists in group %s", child, parent)
	}
	if _, err := tx.Exec(`INSERT INTO group_to_group (parent_group, child_group) VALUES (?, ?)`, parent, child); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) GetAvailableGroupsAsChild(group string) ([]string, error) {
	// TODO(dtabidze): Might have to add further logic to filter available groups as children.
	query := `
		SELECT name FROM groups
		WHERE name != ? AND name NOT IN (
			SELECT child_group FROM group_to_group WHERE parent_group = ?
		)
	`
	rows, err := s.db.Query(query, group, group)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var availableGroups []string
	for rows.Next() {
		var groupName string
		if err := rows.Scan(&groupName); err != nil {
			return nil, err
		}
		availableGroups = append(availableGroups, groupName)
	}
	return availableGroups, nil
}

func getLoggedInUser(r *http.Request) (string, error) {
	// TODO(dtabidze): should make a request to get loggedin user
	return "tabo", nil
}

type Status int

const (
	Owner Status = iota
	Member
)

func convertStatus(status string) (Status, error) {
	switch status {
	case "Owner":
		return Owner, nil
	case "Member":
		return Member, nil
	default:
		return Owner, fmt.Errorf("invalid status: %s", status)
	}
}

func (s *Server) Start() {
	http.Handle("/static/", http.FileServer(http.FS(staticResources)))
	http.HandleFunc("/", s.homePageHandler)
	http.HandleFunc("/group/", s.groupHandler)
	http.HandleFunc("/create-group", s.createGroupHandler)
	http.HandleFunc("/add-user", s.addUserHandler)
	http.HandleFunc("/add-child-group", s.addChildGroupHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

type GroupData struct {
	Group      Group
	Membership string
}

func (s *Server) checkIsOwner(w http.ResponseWriter, user, group string) (bool, error) {
	isOwner, err := s.store.IsGroupOwner(user, group)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return false, err
	}
	if !isOwner {
		http.Error(w, fmt.Sprintf("You are not the owner of the group %s", group), http.StatusUnauthorized)
		return false, nil
	}
	return true, nil
}

func (s *Server) homePageHandler(w http.ResponseWriter, r *http.Request) {
	loggedInUser, err := getLoggedInUser(r)
	if err != nil {
		http.Error(w, "User Not Logged In", http.StatusUnauthorized)
		return
	}
	ownerGroups, err := s.store.GetGroupsOwnedBy(loggedInUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	membershipGroups, err := s.store.GetMembershipGroups(loggedInUser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	tmpl, err := template.New("index").Parse(indexHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		OwnerGroups      []Group
		MembershipGroups []Group
	}{
		OwnerGroups:      ownerGroups,
		MembershipGroups: membershipGroups,
	}
	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) createGroupHandler(w http.ResponseWriter, r *http.Request) {
	loggedInUser, err := getLoggedInUser(r)
	if err != nil {
		http.Error(w, "User Not Logged In", http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var group Group
	group.Name = r.PostFormValue("group-name")
	group.Description = r.PostFormValue("description")
	if err := s.store.CreateGroup(loggedInUser, group); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) groupHandler(w http.ResponseWriter, r *http.Request) {
	groupName := strings.TrimPrefix(r.URL.Path, "/group/")
	tmpl, err := template.New("group").Parse(groupHTML)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	owners, err := s.store.GetGroupOwners(groupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	members, err := s.store.GetGroupMembers(groupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	description, err := s.store.GetGroupDescription(groupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	availableGroups, err := s.store.GetAvailableGroupsAsChild(groupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	data := struct {
		GroupName       string
		Description     string
		Owners          []string
		Members         []string
		AvailableGroups []string
	}{
		GroupName:       groupName,
		Description:     description,
		Owners:          owners,
		Members:         members,
		AvailableGroups: availableGroups,
	}
	if err := tmpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) addUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	loggedInUser, err := getLoggedInUser(r)
	if err != nil {
		http.Error(w, "User Not Logged In", http.StatusUnauthorized)
		return
	}
	groupName := r.FormValue("group")
	username := r.FormValue("username")
	status, err := convertStatus(r.FormValue("status"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, err := s.checkIsOwner(w, loggedInUser, groupName); err != nil {
		return
	}
	switch status {
	case Owner:
		err = s.store.AddGroupOwner(username, groupName)
	case Member:
		err = s.store.AddGroupMember(username, groupName)
	default:
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/group/"+groupName, http.StatusSeeOther)
}

func (s *Server) addChildGroupHandler(w http.ResponseWriter, r *http.Request) {
	// TODO(dtabidze): In future we might need to make one group OWNER of another and not just a member.
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	loggedInUser, err := getLoggedInUser(r)
	if err != nil {
		http.Error(w, "User Not Logged In", http.StatusUnauthorized)
		return
	}
	parentGroup := r.FormValue("parent-group")
	childGroup := r.FormValue("child-group")
	if _, err := s.checkIsOwner(w, loggedInUser, parentGroup); err != nil {
		return
	}
	if err := s.store.AddChildGroup(parentGroup, childGroup); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/group/"+parentGroup, http.StatusSeeOther)
}

func main() {
	flag.Parse()
	db, err := NewSQLiteStore(*dbPath)
	if err != nil {
		panic(err)
	}
	s := Server{store: db}
	s.Start()
}
