package main

type Config struct {
	Values Values `json:"values"`
}

type Values struct {
	PCloudEnvName            string `json:"pcloudEnvName,omitempty"`
	Id                       string `json:"id,omitempty"`
	ContactEmail             string `json:"contactEmail,omitempty"`
	Domain                   string `json:"domain,omitempty"`
	PublicIP                 string `json:"publicIP,omitempty"`
	GandiAPIToken            string `json:"gandiAPIToken,omitempty"`
	NamespacePrefix          string `json:"namespacePrefix,omitempty"`
	LighthouseAuthUIIP       string `json:"lighthouseAuthUIIP,omitempty"`
	LighthouseMainIP         string `json:"lighthouseMainIP,omitempty"`
	LighthouseMainPort       string `json:"lighthouseMainPort,omitempty"`
	MXHostname               string `json:"mxHostname,omitempty"`
	MailGatewayAddress       string `json:"mailGatewayAddress,omitempty"`
	MatrixOAuth2ClientSecret string `json:"matrixOAuth2ClientSecret,omitempty"`
	MatrixStorageSize        string `json:"matrixStorageSize,omitempty"`
	PiholeOAuth2ClientSecret string `json:"piholeOAuth2ClientSecret,omitempty"`
	PiholeOAuth2CookieSecret string `json:"piholeOAuth2CookieSecret,omitempty"`
}
