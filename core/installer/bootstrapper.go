package installer

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"net/netip"
	"path/filepath"
	"time"

	"github.com/cenkalti/backoff/v4"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"

	"github.com/giolekva/pcloud/core/installer/soft"
)

const IPAddressPoolLocal = "local"
const IPAddressPoolConfigRepo = "config-repo"
const IPAddressPoolIngressPublic = "ingress-public"

type Bootstrapper struct {
	cl ChartLoader
	ns NamespaceCreator
	ha HelmActionConfigFactory
}

func NewBootstrapper(cl ChartLoader, ns NamespaceCreator, ha HelmActionConfigFactory) Bootstrapper {
	return Bootstrapper{cl, ns, ha}
}

func (b Bootstrapper) Run(env EnvConfig) error {
	bootstrapJobKeys, err := NewSSHKeyPair()
	if err != nil {
		return err
	}
	if err := b.installMetallb(env); err != nil {
		return err
	}
	if err := b.installLonghorn(env.Name, env.StorageDir, env.VolumeDefaultReplicaCount); err != nil {
		return err
	}
	time.Sleep(1 * time.Minute) // TODO(giolekva): implement proper wait
	if err := b.installSoftServe(bootstrapJobKeys.Public, env.Name, env.ServiceIPs.ConfigRepo); err != nil {
		return err
	}
	var ss *soft.Client
	err = backoff.Retry(func() error {
		var err error
		ss, err = soft.NewClient(netip.AddrPortFrom(env.ServiceIPs.ConfigRepo, 22), []byte(bootstrapJobKeys.Private), log.Default())
		return err
	}, backoff.NewConstantBackOff(5*time.Second))
	if err != nil {
		return err
	}
	if ss.AddPublicKey("admin", string(env.AdminPublicKey)); err != nil {
		return err
	}
	if err := b.installFluxcd(ss, env.Name); err != nil {
		return err
	}
	repo, err := ss.GetRepo(env.Name)
	if err != nil {
		return err
	}
	repoIO := NewRepoIO(repo, ss.Signer)
	if err := configureMainRepo(repoIO, env); err != nil {
		return err
	}
	nsGen := NewPrefixGenerator(env.NamespacePrefix)
	if err := b.installInfrastructureServices(repoIO, nsGen, b.ns, env); err != nil {
		return err
	}
	if err := b.installEnvManager(ss, repoIO, nsGen, b.ns, env); err != nil {
		return err
	}
	if ss.RemovePublicKey("admin", bootstrapJobKeys.Public); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installMetallb(env EnvConfig) error {
	if err := b.installMetallbNamespace(env); err != nil {
		return err
	}
	if err := b.installMetallbService(); err != nil {
		return err
	}
	if err := b.installMetallbIPAddressPool(IPAddressPoolLocal, true, env.ServiceIPs.From, env.ServiceIPs.To); err != nil {
		return err
	}
	if err := b.installMetallbIPAddressPool(IPAddressPoolConfigRepo, false, env.ServiceIPs.ConfigRepo, env.ServiceIPs.ConfigRepo); err != nil {
		return err
	}
	if err := b.installMetallbIPAddressPool(IPAddressPoolIngressPublic, false, env.ServiceIPs.IngressPublic, env.ServiceIPs.IngressPublic); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installMetallbNamespace(env EnvConfig) error {
	fmt.Println("Installing metallb namespace")
	config, err := b.ha.New(env.Name)
	if err != nil {
		return err
	}
	chart, err := b.cl.Load("namespace")
	if err != nil {
		return err
	}
	values := map[string]any{
		"namespace": "metallb-system",
		"labels": []string{
			"pod-security.kubernetes.io/audit: privileged",
			"pod-security.kubernetes.io/enforce: privileged",
			"pod-security.kubernetes.io/warn: privileged",
		},
	}
	installer := action.NewInstall(config)
	installer.Namespace = env.Name
	installer.ReleaseName = "metallb-ns"
	installer.Wait = true
	installer.WaitForJobs = true
	if _, err := installer.RunWithContext(context.TODO(), chart, values); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installMetallbService() error {
	fmt.Println("Installing metallb")
	config, err := b.ha.New("metallb-system")
	if err != nil {
		return err
	}
	chart, err := b.cl.Load("metallb")
	if err != nil {
		return err
	}
	values := map[string]any{ // TODO(giolekva): add loadBalancerClass?
		"controller": map[string]any{
			"image": map[string]any{
				"repository": "quay.io/metallb/controller",
				"tag":        "v0.13.9",
				"pullPolicy": "IfNotPresent",
			},
			"logLevel": "info",
		},
		"speaker": map[string]any{
			"image": map[string]any{
				"repository": "quay.io/metallb/speaker",
				"tag":        "v0.13.9",
				"pullPolicy": "IfNotPresent",
			},
			"logLevel": "info",
		},
	}
	installer := action.NewInstall(config)
	installer.Namespace = "metallb-system"
	installer.CreateNamespace = true
	installer.ReleaseName = "metallb"
	installer.IncludeCRDs = true
	installer.Wait = true
	installer.WaitForJobs = true
	installer.Timeout = 20 * time.Minute
	if _, err := installer.RunWithContext(context.TODO(), chart, values); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installMetallbIPAddressPool(name string, autoAssign bool, from, to netip.Addr) error {
	fmt.Printf("Installing metallb-ipaddresspool: %s\n", name)
	config, err := b.ha.New("metallb-system")
	if err != nil {
		return err
	}
	chart, err := b.cl.Load("metallb-ipaddresspool")
	if err != nil {
		return err
	}
	values := map[string]any{
		"name":       name,
		"autoAssign": autoAssign,
		"from":       from.String(),
		"to":         to.String(),
	}
	installer := action.NewInstall(config)
	installer.Namespace = "metallb-system"
	installer.CreateNamespace = true
	installer.ReleaseName = name
	installer.Wait = true
	installer.WaitForJobs = true
	installer.Timeout = 20 * time.Minute
	if _, err := installer.RunWithContext(context.TODO(), chart, values); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installLonghorn(envName string, storageDir string, volumeDefaultReplicaCount int) error {
	fmt.Println("Installing Longhorn")
	config, err := b.ha.New(envName)
	if err != nil {
		return err
	}
	chart, err := b.cl.Load("longhorn")
	if err != nil {
		return err
	}
	values := map[string]any{
		"defaultSettings": map[string]any{
			"defaultDataPath": storageDir,
		},
		"persistence": map[string]any{
			"defaultClassReplicaCount": volumeDefaultReplicaCount,
		},
		"service": map[string]any{
			"ui": map[string]any{
				"type": "LoadBalancer",
			},
		},
		"ingress": map[string]any{
			"enabled": false,
		},
	}
	installer := action.NewInstall(config)
	installer.Namespace = "longhorn-system"
	installer.CreateNamespace = true
	installer.ReleaseName = "longhorn"
	installer.Wait = true
	installer.WaitForJobs = true
	installer.Timeout = 20 * time.Minute
	if _, err := installer.RunWithContext(context.TODO(), chart, values); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installSoftServe(adminPublicKey string, envName string, repoIP netip.Addr) error {
	fmt.Println("Installing SoftServe")
	keys, err := NewSSHKeyPair()
	if err != nil {
		return err
	}
	config, err := b.ha.New(envName)
	if err != nil {
		return err
	}
	chart, err := b.cl.Load("soft-serve")
	if err != nil {
		return err
	}
	values := map[string]any{
		"image": map[string]any{
			"repository": "charmcli/soft-serve",
			"tag":        "v0.5.4",
			"pullPolicy": "IfNotPresent",
		},
		"privateKey": keys.Private,
		"publicKey":  keys.Public,
		"adminKey":   adminPublicKey,
		"reservedIP": repoIP.String(),
	}
	installer := action.NewInstall(config)
	installer.Namespace = envName
	installer.CreateNamespace = true
	installer.ReleaseName = "soft-serve"
	installer.Wait = true
	installer.WaitForJobs = true
	installer.Timeout = 20 * time.Minute
	if _, err := installer.RunWithContext(context.TODO(), chart, values); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installFluxcd(ss *soft.Client, envName string) error {
	keys, err := NewSSHKeyPair()
	if err != nil {
		return err
	}
	if err := ss.AddUser("flux", keys.Public); err != nil {
		return err
	}
	if err := ss.MakeUserAdmin("flux"); err != nil {
		return err
	}
	fmt.Printf("Creating /%s repo", envName)
	if err := ss.AddRepository(envName, "# dodo Systems"); err != nil {
		return err
	}
	fmt.Println("Installing Flux")
	ssPublic, err := ss.GetPublicKey()
	if err != nil {
		return err
	}
	if err := b.installFluxBootstrap(
		ss.GetRepoAddress(envName),
		ss.Addr.Addr().String(),
		string(ssPublic),
		keys.Private,
		envName,
	); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installFluxBootstrap(repoAddr, repoHost, repoHostPubKey, privateKey, envName string) error {
	config, err := b.ha.New(envName)
	if err != nil {
		return err
	}
	chart, err := b.cl.Load("flux-bootstrap")
	if err != nil {
		return err
	}
	values := map[string]any{
		"image": map[string]any{
			"repository": "fluxcd/flux-cli", // "giolekva/flux",
			"tag":        "v2.0.0",
			"pullPolicy": "IfNotPresent",
		},
		"repositoryAddress":       repoAddr,
		"repositoryHost":          repoHost,
		"repositoryHostPublicKey": repoHostPubKey,
		"privateKey":              privateKey,
		"installationNamespace":   fmt.Sprintf("%s-flux", envName),
	}
	installer := action.NewInstall(config)
	installer.Namespace = envName
	installer.CreateNamespace = true
	installer.ReleaseName = "flux"
	installer.Wait = true
	installer.WaitForJobs = true
	installer.Timeout = 20 * time.Minute
	if _, err := installer.RunWithContext(context.TODO(), chart, values); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installInfrastructureServices(repo RepoIO, nsGen NamespaceGenerator, nsCreator NamespaceCreator, env EnvConfig) error {
	appRepo := NewInMemoryAppRepository(CreateAllApps())
	install := func(name string) error {
		app, err := appRepo.Find(name)
		if err != nil {
			return err
		}
		namespaces := make([]string, len(app.Namespaces))
		for i, n := range app.Namespaces {
			namespaces[i], err = nsGen.Generate(n)
			if err != nil {
				return err
			}
		}
		for _, n := range namespaces {
			if err := nsCreator.Create(n); err != nil {
				return err
			}
		}
		derived := Derived{
			Global: Values{
				PCloudEnvName: env.Name,
			},
		}
		if len(namespaces) > 0 {
			derived.Release.Namespace = namespaces[0]
		}
		values := map[string]any{
			"IngressPublicIP": env.ServiceIPs.IngressPublic.String(),
		}
		return repo.InstallApp(*app, filepath.Join("/infrastructure", app.Name), values, derived)
	}
	appsToInstall := []string{
		"resource-renderer-controller",
		"headscale-controller",
		"csi-driver-smb",
		"ingress-public",
		"cert-manager",
		"cert-manager-webhook-gandi",
		"cert-manager-webhook-gandi-role",
	}
	for _, name := range appsToInstall {
		if err := install(name); err != nil {
			return err
		}
	}
	return nil
}

func configureMainRepo(repo RepoIO, env EnvConfig) error {
	if err := repo.WriteYaml("config.yaml", env); err != nil {
		return err
	}
	kust := NewKustomization()
	kust.AddResources(
		fmt.Sprintf("%s-flux", env.Name),
		"infrastructure",
		"environments",
	)
	if err := repo.WriteKustomization("kustomization.yaml", kust); err != nil {
		return err
	}
	{
		out, err := repo.Writer("infrastructure/pcloud-charts.yaml")
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = out.Write([]byte(fmt.Sprintf(`
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: pcloud # TODO(giolekva): use more generic name
  namespace: %s
spec:
  interval: 1m0s
  url: https://github.com/giolekva/pcloud
  ref:
    branch: main
`, env.Name)))
		if err != nil {
			return err
		}
	}
	infraKust := NewKustomization()
	infraKust.AddResources("pcloud-charts.yaml")
	if err := repo.WriteKustomization("infrastructure/kustomization.yaml", infraKust); err != nil {
		return err
	}
	if err := repo.WriteKustomization("environments/kustomization.yaml", NewKustomization()); err != nil {
		return err
	}
	if err := repo.CommitAndPush("initialize pcloud directory structure"); err != nil {
		return err
	}
	return nil
}

func (b Bootstrapper) installEnvManager(ss *soft.Client, repo RepoIO, nsGen NamespaceGenerator, nsCreator NamespaceCreator, env EnvConfig) error {
	keys, err := NewSSHKeyPair()
	if err != nil {
		return err
	}
	user := fmt.Sprintf("%s-env-manager", env.Name)
	if err := ss.AddUser(user, keys.Public); err != nil {
		return err
	}
	if err := ss.MakeUserAdmin(user); err != nil {
		return err
	}
	appRepo := NewInMemoryAppRepository(CreateAllApps())
	app, err := appRepo.Find("env-manager")
	if err != nil {
		return err
	}
	namespaces := make([]string, len(app.Namespaces))
	for i, n := range app.Namespaces {
		namespaces[i], err = nsGen.Generate(n)
		if err != nil {
			return err
		}
	}
	for _, n := range namespaces {
		if err := nsCreator.Create(n); err != nil {
			return err
		}
	}
	derived := Derived{
		Global: Values{
			PCloudEnvName: env.Name,
		},
		Values: map[string]any{
			"RepoIP":        env.ServiceIPs.ConfigRepo,
			"RepoPort":      22,
			"RepoName":      env.Name,
			"SSHPrivateKey": keys.Private,
		},
	}
	if len(namespaces) > 0 {
		derived.Release.Namespace = namespaces[0]
	}
	return repo.InstallApp(*app, filepath.Join("/infrastructure", app.Name), derived.Values, derived)
}

type HelmActionConfigFactory interface {
	New(namespace string) (*action.Configuration, error)
}

type ChartLoader interface {
	Load(name string) (*chart.Chart, error)
}

type fsChartLoader struct {
	baseDir string
}

func NewFSChartLoader(baseDir string) ChartLoader {
	return &fsChartLoader{baseDir}
}

func (l *fsChartLoader) Load(name string) (*chart.Chart, error) {
	return loader.Load(filepath.Join(l.baseDir, name))
}