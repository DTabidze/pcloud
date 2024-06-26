package main

import (
	_ "embed"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/giolekva/pcloud/core/installer"
	"github.com/giolekva/pcloud/core/installer/soft"
)

var bootstrapFlags struct {
	envName                   string
	publicIP                  string
	chartsDir                 string
	adminPubKey               string
	storageDir                string
	volumeDefaultReplicaCount int
	fromIP                    string
	toIP                      string
}

func bootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "bootstrap",
		RunE: bootstrapCmdRun,
	}
	cmd.Flags().StringVar(
		&bootstrapFlags.envName,
		"env-name",
		"pcloud",
		"",
	)
	cmd.Flags().StringVar(
		&bootstrapFlags.publicIP,
		"public-ip",
		"",
		"",
	)
	cmd.Flags().StringVar(
		&bootstrapFlags.chartsDir,
		"charts-dir",
		"",
		"",
	)
	cmd.Flags().StringVar(
		&bootstrapFlags.adminPubKey,
		"admin-pub-key",
		"",
		"",
	)
	cmd.Flags().StringVar(
		&bootstrapFlags.storageDir,
		"storage-dir",
		"",
		"",
	)
	cmd.Flags().IntVar(
		&bootstrapFlags.volumeDefaultReplicaCount,
		"volume-default-replica-count",
		3,
		"",
	)
	cmd.Flags().StringVar(
		&bootstrapFlags.fromIP,
		"from-ip",
		"",
		"",
	)
	cmd.Flags().StringVar(
		&bootstrapFlags.toIP,
		"to-ip",
		"",
		"",
	)
	return cmd
}

func bootstrapCmdRun(cmd *cobra.Command, args []string) error {
	// TODO(gio): remove
	installer.CreateAllApps()
	adminPubKey, err := os.ReadFile(bootstrapFlags.adminPubKey)
	if err != nil {
		return err
	}
	nsCreator, err := newNSCreator()
	if err != nil {
		return err
	}
	serviceIPs, err := newServiceIPs(bootstrapFlags.fromIP, bootstrapFlags.toIP)
	if err != nil {
		return err
	}
	publicIPs, err := parseIPs(bootstrapFlags.publicIP)
	envConfig := installer.BootstrapConfig{
		InfraName:                 bootstrapFlags.envName,
		PublicIP:                  publicIPs,
		NamespacePrefix:           fmt.Sprintf("%s-", bootstrapFlags.envName),
		StorageDir:                bootstrapFlags.storageDir,
		VolumeDefaultReplicaCount: bootstrapFlags.volumeDefaultReplicaCount,
		AdminPublicKey:            adminPubKey,
		ServiceIPs:                serviceIPs,
	}
	b := installer.NewBootstrapper(
		installer.NewFSChartLoader(bootstrapFlags.chartsDir),
		nsCreator,
		installer.NewActionConfigFactory(rootFlags.kubeConfig),
		installer.NewInMemoryAppRepository(installer.CreateAllApps()),
		soft.RealClientGetter{},
	)
	return b.Run(envConfig)
}

func newServiceIPs(from, to string) (installer.EnvServiceIPs, error) {
	f, err := netip.ParseAddr(from)
	if err != nil {
		return installer.EnvServiceIPs{}, err
	}
	t, err := netip.ParseAddr(to)
	if err != nil {
		return installer.EnvServiceIPs{}, err
	}
	configRepo := f
	ingressPublic := configRepo.Next()
	restFrom := ingressPublic.Next()
	return installer.EnvServiceIPs{
		ConfigRepo:    configRepo,
		IngressPublic: ingressPublic,
		From:          restFrom,
		To:            t,
	}, nil
}

func parseIPs(ip string) ([]net.IP, error) {
	ret := make([]net.IP, 0)
	for _, i := range strings.Split(ip, ",") {
		ip := net.ParseIP(i)
		if ip == nil {
			return nil, fmt.Errorf("invalid ip: %s", i)
		}
		ret = append(ret, ip)
	}
	return ret, nil
}
