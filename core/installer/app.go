package main

import "text/template"

type App struct {
	Name      string
	Templates []*template.Template
}

func CreateAppIngressPrivate(tmpls *template.Template) App {
	return App{
		"ingress-private",
		[]*template.Template{
			tmpls.Lookup("vpn-mesh-config.yaml"),
			tmpls.Lookup("ingress-private.yaml"),
			tmpls.Lookup("certificate-issuer.yaml"),
		},
	}
}

func CreateAppCoreAuth(tmpls *template.Template) App {
	return App{
		"core-auth",
		[]*template.Template{
			tmpls.Lookup("core-auth-storage.yaml"),
			tmpls.Lookup("core-auth.yaml"),
		},
	}
}

func CreateAppVaultwarden(tmpls *template.Template) App {
	return App{
		"vaultwarden",
		[]*template.Template{
			tmpls.Lookup("vaultwarden.yaml"),
		},
	}
}

func CreateAppMatrix(tmpls *template.Template) App {
	return App{
		"matrix",
		[]*template.Template{
			tmpls.Lookup("matrix-storage.yaml"),
			tmpls.Lookup("matrix.yaml"),
		},
	}
}

func CreateAppPihole(tmpls *template.Template) App {
	return App{
		"pihole",
		[]*template.Template{
			tmpls.Lookup("pihole.yaml"),
		},
	}
}

func CreateAppMaddy(tmpls *template.Template) App {
	return App{
		"maddy",
		[]*template.Template{
			tmpls.Lookup("maddy.yaml"),
		},
	}
}