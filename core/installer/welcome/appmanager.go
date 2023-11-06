package welcome

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	// "net/http/httputil"
	// "net/url"

	"github.com/Masterminds/sprig/v3"
	"github.com/labstack/echo/v4"

	"github.com/giolekva/pcloud/core/installer"
)

//go:embed appmanager-tmpl
var mgrTmpl embed.FS

//go:embed appmanager-tmpl/base.html
var baseHtmlTmpl string

//go:embed appmanager-tmpl/app.html
var appHtmlTmpl string

type AppManagerServer struct {
	port       int
	webAppAddr string
	m          *installer.AppManager
	r          installer.AppRepository[installer.StoreApp]
}

func NewAppManagerServer(
	port int,
	webAppAddr string,
	m *installer.AppManager,
	r installer.AppRepository[installer.StoreApp],
) *AppManagerServer {
	return &AppManagerServer{
		port,
		webAppAddr,
		m,
		r,
	}
}

func (s *AppManagerServer) Start() {
	e := echo.New()
	e.StaticFS("/static", echo.MustSubFS(staticAssets, "static"))
	e.GET("/api/app-repo", s.handleAppRepo)
	e.POST("/api/app/:slug/render", s.handleAppRender)
	e.POST("/api/app/:slug/install", s.handleAppInstall)
	e.GET("/api/app/:slug", s.handleApp)
	e.GET("/api/instance/:slug", s.handleInstance)
	e.POST("/api/instance/:slug/update", s.handleAppUpdate)
	e.POST("/api/instance/:slug/remove", s.handleAppRemove)
	e.GET("/", s.handleIndex)
	e.GET("/app/:slug", s.handleAppUI)
	e.GET("/instance/:slug", s.handleInstanceUI)
	fmt.Printf("Starting HTTP server on port: %d\n", s.port)
	log.Fatal(e.Start(fmt.Sprintf(":%d", s.port)))
}

type app struct {
	Name             string                `json:"name"`
	Icon             template.HTML         `json:"icon"`
	ShortDescription string                `json:"shortDescription"`
	Slug             string                `json:"slug"`
	Schema           string                `json:"schema"`
	Instances        []installer.AppConfig `json:"instances,omitempty"`
}

func (s *AppManagerServer) handleAppRepo(c echo.Context) error {
	all, err := s.r.GetAll()
	if err != nil {
		return err
	}
	resp := make([]app, len(all))
	for i, a := range all {
		resp[i] = app{a.Name, a.Icon, a.ShortDescription, a.Name, a.Schema, nil}
	}
	return c.JSON(http.StatusOK, resp)
}

func (s *AppManagerServer) handleApp(c echo.Context) error {
	slug := c.Param("slug")
	a, err := s.r.Find(slug)
	if err != nil {
		return err
	}
	instances, err := s.m.FindAllInstances(slug)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		values, ok := instance.Config["Values"].(map[string]any)
		if !ok {
			return fmt.Errorf("Expected map")
		}
		for k, v := range values {
			if k == "Network" {
				n, ok := v.(map[string]any)
				if !ok {
					return fmt.Errorf("Expected map")
				}
				values["Network"], ok = n["Name"]
				if !ok {
					return fmt.Errorf("Missing Name")
				}
				break
			}
		}
	}
	return c.JSON(http.StatusOK, app{a.Name, a.Icon, a.ShortDescription, a.Name, a.Schema, instances})
}

func (s *AppManagerServer) handleInstance(c echo.Context) error {
	slug := c.Param("slug")
	instance, err := s.m.FindInstance(slug)
	if err != nil {
		return err
	}
	values, ok := instance.Config["Values"].(map[string]any)
	if !ok {
		return fmt.Errorf("Expected map")
	}
	for k, v := range values {
		if k == "Network" {
			n, ok := v.(map[string]any)
			if !ok {
				return fmt.Errorf("Expected map")
			}
			values["Network"], ok = n["Name"]
			if !ok {
				return fmt.Errorf("Missing Name")
			}
			break
		}
	}
	a, err := s.r.Find(instance.AppId)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, app{a.Name, a.Icon, a.ShortDescription, a.Name, a.Schema, []installer.AppConfig{instance}})
}

type file struct {
	Name     string `json:"name"`
	Contents string `json:"contents"`
}

type rendered struct {
	Readme string `json:"readme"`
	Files  []file `json:"files"`
}

func (s *AppManagerServer) handleAppRender(c echo.Context) error {
	slug := c.Param("slug")
	contents, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	global, err := s.m.Config()
	if err != nil {
		return err
	}
	var values map[string]any
	if err := json.Unmarshal(contents, &values); err != nil {
		return err
	}
	if network, ok := values["Network"]; ok {
		for _, n := range installer.CreateNetworks(global) {
			if n.Name == network { // TODO(giolekva): handle not found
				values["Network"] = n
			}
		}
	}
	all := map[string]any{
		"Global": global.Values,
		"Values": values,
	}
	a, err := s.r.Find(slug)
	if err != nil {
		return err
	}
	var readme bytes.Buffer
	if err := a.Readme.Execute(&readme, all); err != nil {
		return err
	}
	var resp rendered
	resp.Readme = readme.String()
	for _, tmpl := range a.Templates { // TODO(giolekva): deduplicate with Install
		var f bytes.Buffer
		if err := tmpl.Execute(&f, all); err != nil {
			fmt.Printf("%+v\n", all)
			fmt.Println(err.Error())
			return err
		} else {
			resp.Files = append(resp.Files, file{tmpl.Name(), f.String()})
		}
	}
	out, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := c.Response().Writer.Write(out); err != nil {
		return err
	}
	return nil
}

func (s *AppManagerServer) handleAppInstall(c echo.Context) error {
	slug := c.Param("slug")
	contents, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	var values map[string]any
	if err := json.Unmarshal(contents, &values); err != nil {
		return err
	}
	fmt.Println(values)
	a, err := s.r.Find(slug)
	if err != nil {
		return err
	}
	config, err := s.m.Config()
	if err != nil {
		return err
	}
	nsGen := installer.NewPrefixGenerator(config.Values.NamespacePrefix)
	suffixGen := installer.NewFixedLengthRandomSuffixGenerator(3)
	if err := s.m.Install(a.App, nsGen, suffixGen, values); err != nil {
		return err
	}
	return c.String(http.StatusOK, "Installed")
}

func (s *AppManagerServer) handleAppUpdate(c echo.Context) error {
	slug := c.Param("slug")
	appConfig, err := s.m.AppConfig(slug)
	if err != nil {
		return err
	}
	contents, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	var values map[string]any
	if err := json.Unmarshal(contents, &values); err != nil {
		return err
	}
	a, err := s.r.Find(appConfig.AppId)
	if err != nil {
		return err
	}
	if err := s.m.Update(a.App, slug, values); err != nil {
		return err
	}
	return c.String(http.StatusOK, "Installed")
}

func (s *AppManagerServer) handleAppRemove(c echo.Context) error {
	slug := c.Param("slug")
	if err := s.m.Remove(slug); err != nil {
		return err
	}
	return c.String(http.StatusOK, "Installed")
}

func (s *AppManagerServer) handleIndex(c echo.Context) error {
	tmpl, err := template.ParseFS(mgrTmpl, "appmanager-tmpl/base.html", "appmanager-tmpl/index.html")
	if err != nil {
		return err
	}
	all, err := s.r.GetAll()
	if err != nil {
		return err
	}
	resp := make([]app, len(all))
	for i, a := range all {
		resp[i] = app{a.Name, a.Icon, a.ShortDescription, a.Name, a.Schema, nil}
	}
	return tmpl.Execute(c.Response(), resp)
}

type appContext[T any] struct {
	App               *T
	Instance          *installer.AppConfig
	Instances         []installer.AppConfig
	AvailableNetworks []installer.Network
}

func (s *AppManagerServer) handleAppUI(c echo.Context) error {
	baseTmpl, err := newTemplate().Parse(baseHtmlTmpl)
	if err != nil {
		return err
	}
	appTmpl, err := template.Must(baseTmpl.Clone()).Parse(appHtmlTmpl)
	if err != nil {
		fmt.Println(err)
		return err
	}
	global, err := s.m.Config()
	if err != nil {
		return err
	}
	slug := c.Param("slug")
	a, err := s.r.Find(slug)
	if err != nil {
		return err
	}
	instances, err := s.m.FindAllInstances(slug)
	if err != nil {
		return err
	}
	err = appTmpl.Execute(c.Response(), appContext[installer.StoreApp]{
		App:               a,
		Instances:         instances,
		AvailableNetworks: installer.CreateNetworks(global),
	})
	fmt.Println(err)
	return err
}

func (s *AppManagerServer) handleInstanceUI(c echo.Context) error {
	baseTmpl, err := newTemplate().Parse(baseHtmlTmpl)
	if err != nil {
		return err
	}
	appTmpl, err := template.Must(baseTmpl.Clone()).Parse(appHtmlTmpl)
	// tmpl, err := newTemplate().ParseFS(mgrTmpl, "appmanager-tmpl/base.html", "appmanager-tmpl/app.html")
	if err != nil {
		fmt.Println(err)
		return err
	}
	global, err := s.m.Config()
	if err != nil {
		return err
	}
	slug := c.Param("slug")
	instance, err := s.m.FindInstance(slug)
	if err != nil {
		return err
	}
	a, err := s.r.Find(instance.AppId)
	if err != nil {
		return err
	}
	instances, err := s.m.FindAllInstances(a.Name)
	if err != nil {
		return err
	}
	err = appTmpl.Execute(c.Response(), appContext[installer.StoreApp]{
		App:               a,
		Instance:          &instance,
		Instances:         instances,
		AvailableNetworks: installer.CreateNetworks(global),
	})
	fmt.Println(err)
	return err
}

func newTemplate() *template.Template {
	return template.New("base").Funcs(template.FuncMap(sprig.FuncMap()))
}