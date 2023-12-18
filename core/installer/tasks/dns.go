package tasks

import (
	"context"
	"fmt"
	"net"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"

	"github.com/giolekva/pcloud/core/installer"
)

type Check func(ch Check) error

func SetupZoneTask(env Env, st *state) Task {
	return newSequentialParentTask(
		fmt.Sprintf("Setup DNS zone records for %s", env.Domain),
		CreateZoneRecords(env.Domain, st.publicIPs, env, st),
		WaitToPropagate(env.Domain, st.publicIPs),
	)
}

func CreateZoneRecords(
	name string,
	expected []net.IP,
	env Env,
	st *state,
) Task {
	t := newLeafTask("Configure DNS", func() error {
		repo, err := st.ssClient.GetRepo("config")
		if err != nil {
			return err
		}
		r := installer.NewRepoIO(repo, st.ssClient.Signer)
		{
			key, err := newDNSSecKey(env.Domain)
			if err != nil {
				return err
			}
			out, err := r.Writer("dns-zone.yaml")
			if err != nil {
				return err
			}
			defer out.Close()
			dnsZoneTmpl, err := template.New("config").Funcs(sprig.TxtFuncMap()).Parse(`
apiVersion: dodo.cloud.dodo.cloud/v1
kind: DNSZone
metadata:
  name: dns-zone
  namespace: {{ .namespace }}
spec:
  zone: {{ .zone }}
  privateIP: 10.1.0.1
  publicIPs:
{{ range .publicIPs }}
  - {{ .String }}
{{ end }}
  nameservers:
{{ range .publicIPs }}
  - {{ .String }}
{{ end }}
  dnssec:
    enabled: true
    secretName: dnssec-key
---
apiVersion: v1
kind: Secret
metadata:
  name: dnssec-key
  namespace: {{ .namespace }}
type: Opaque
data:
  basename: {{ .dnssec.Basename | b64enc }}
  key: {{ .dnssec.Key | toString | b64enc }}
  private: {{ .dnssec.Private | toString | b64enc }}
  ds: {{ .dnssec.DS | toString | b64enc }}
`)
			if err != nil {
				return err
			}
			if err := dnsZoneTmpl.Execute(out, map[string]any{
				"namespace": env.Name,
				"zone":      env.Domain,
				"dnssec":    key,
				"publicIPs": st.publicIPs,
			}); err != nil {
				return err
			}
			rootKust := installer.NewKustomization()
			rootKust.AddResources("dns-zone.yaml")
			if err := r.WriteKustomization("kustomization.yaml", rootKust); err != nil {
				return err
			}
			r.CommitAndPush("configure dns zone")
		}
		return nil
	})
	return &t
}

func WaitToPropagate(
	name string,
	expected []net.IP,
) Task {
	t := newLeafTask("Propagate DNS records", func() error {
		ctx := context.TODO()
		gotExpectedIPs := func(actual []net.IP) bool {
			for _, a := range actual {
				found := false
				for _, e := range expected {
					if a.Equal(e) {
						found = true
						break
					}
				}
				if !found {
					return false
				}
			}
			return true
		}
		check := func(check Check) error {
			addrs, err := net.LookupIP(name)
			fmt.Printf("DNS LOOKUP: %+v\n", addrs)
			if err == nil && gotExpectedIPs(addrs) {
				return err
			}
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(5 * time.Second):
				return check(check)
			}
		}
		return check(check)
	})
	return &t
}