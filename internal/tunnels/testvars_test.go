package tunnels

import "github.com/lanjelin/sishc/internal/testvars"

var (
	tunnelRemoteServer = testvars.String("SISHC_TEST_REMOTE_SERVER", "example.test")
	tunnelRemotePort   = testvars.Int("SISHC_TEST_REMOTE_PORT", 2222)
	tunnelSubdomain    = testvars.String("SISHC_TEST_SUBDOMAIN", "sub.example.test")
	tunnelPublicIP     = testvars.String("SISHC_TEST_PUBLIC_IP", "198.51.100.10")
	tunnelSSHKey       = testvars.String("SISHC_TEST_SSH_KEY", "~/.ssh/id_rsa")
)
