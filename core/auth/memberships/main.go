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
var f embed.FS

type Store interface {
	CreateGroup(owner string, group Group) error
	GetGroupsOwnedBy(user string) ([]Group, error)
	GetMembershipGroups(user string) ([]Group, error)
	IsGroupOwner(user, group string) (bool, error)
	AddGroupMember(user, group string) error
	AddGroupOwner(user, group string) error
	GetGroupOwners(group string) ([]string, error)
	GetGroupMembers(group string) ([]string, error)
	GetGroupDescription(groupName string) (string, error)
	userGroupPairExists(table, user, group string) (bool, error)
}

type Server struct {
	store Store
}

type Group struct {
	Name        string
	Description string
}

type Member struct {
	Username string
	Groups   []Group
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
	query := `INSERT INTO groups (name, description) VALUES (?, ?)`
	_, err := s.db.Exec(query, group.Name, group.Description)
	if err != nil {
		sqliteErr, ok := err.(*sqlite3.Error)
		if ok && sqliteErr.ExtendedCode() == 1555 {
			return fmt.Errorf("Group with the name %s already exists", group.Name)
		}
		// TODO(dtabidze): have to research go-sqlite3 lib to handle errors better
		// if ok && sqliteErr.Code == sqlite.ErrConstraintUnique {
		// 	return fmt.Errorf("Group with the name %s already exists", group.Name)
		// }
		return err
	}
	query = `INSERT INTO owners (username, group_name) VALUES (?, ?)`
	_, err = s.db.Exec(query, owner, group.Name)
	if err != nil {
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
	err := s.db.QueryRow(query, user, group).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SQLiteStore) userGroupPairExists(table, user, group string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE username = ? AND group_name = ?)", table)
	var exists bool
	err := s.db.QueryRow(query, user, group).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s *SQLiteStore) AddGroupMember(user, group string) error {
	existsInUserToGroup, err := s.userGroupPairExists("user_to_group", user, group)
	if err != nil {
		return err
	}
	if existsInUserToGroup {
		return fmt.Errorf("%s is already a member of group %s", user, group)
	}
	_, err = s.db.Exec(`INSERT INTO user_to_group (username, group_name) VALUES (?, ?)`, user, group)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) AddGroupOwner(user, group string) error {
	existsInOwners, err := s.userGroupPairExists("owners", user, group)
	if err != nil {
		return err
	}
	if existsInOwners {
		return fmt.Errorf("%s is already an owner of group %s", user, group)
	}
	_, err = s.db.Exec(`INSERT INTO owners (username, group_name) VALUES (?, ?)`, user, group)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLiteStore) getUsersByGroup(table, groupname string) ([]string, error) {
	query := fmt.Sprintf("SELECT username FROM %s WHERE group_name = ?", table)
	rows, err := s.db.Query(query, groupname)
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

func (s *SQLiteStore) GetGroupDescription(groupName string) (string, error) {
	var description string
	query := `SELECT description FROM groups WHERE name = ?`
	err := s.db.QueryRow(query, groupName).Scan(&description)
	if err != nil {
		return "", err
	}
	return description, nil
}

func getLoggedInUser(r *http.Request) (string, error) {
	// TODO(dtabidze): should make a request to get loggedin user
	return "tabo", nil
}

func (s *Server) Start() {
	http.Handle("/static/", http.FileServer(http.FS(f)))
	http.HandleFunc("/", s.homePageHandler)
	http.HandleFunc("/group/", s.groupHandler)
	http.HandleFunc("/adduser", s.addUserHandler)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

type GroupData struct {
	Group      Group
	Membership string
}

func (s *Server) homePageHandler(w http.ResponseWriter, r *http.Request) {
	loggedInUser, err := getLoggedInUser(r)
	if err != nil {
		http.Error(w, "User Not Logged In", http.StatusUnauthorized)
		return
	}
	if r.Method == http.MethodPost && r.URL.Path == "/create-group" {
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
	err = tmpl.Execute(w, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
	data := struct {
		GroupName   string
		Description string
		Owners      []string
		Members     []string
	}{
		GroupName:   groupName,
		Description: description,
		Owners:      owners,
		Members:     members,
	}
	err = tmpl.Execute(w, data)
	if err != nil {
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
	status := r.FormValue("status")
	isOwner, err := s.store.IsGroupOwner(loggedInUser, groupName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !isOwner {
		http.Error(w, fmt.Sprintf("You are not the owner of the group %s", groupName), http.StatusUnauthorized)
		return
	}
	switch status {
	case "Owner":
		err = s.store.AddGroupOwner(username, groupName)
	case "Member":
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

func main() {
	flag.Parse()
	db, err := NewSQLiteStore(*dbPath)
	if err != nil {
		panic(err)
	}
	s := Server{store: db}
	s.Start()
}
