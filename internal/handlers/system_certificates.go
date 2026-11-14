package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"github.com/msVPNco/rovunet-panel/internal/actions"
	"github.com/msVPNco/rovunet-panel/internal/certs"
	"github.com/msVPNco/rovunet-panel/internal/config"
	"github.com/msVPNco/rovunet-panel/internal/coreinstall"
	"github.com/msVPNco/rovunet-panel/internal/dnsrouter"
	"github.com/msVPNco/rovunet-panel/internal/prompt"
	"github.com/msVPNco/rovunet-panel/internal/service"
	"github.com/msVPNco/rovunet-panel/internal/transport"
	"os"
	"path/filepath"
	"time"
)

func certificatePair(cert, key, domain string, now time.Time) (*x509.Certificate, error) {
	pair, e := tls.LoadX509KeyPair(cert, key)
	if e != nil || len(pair.Certificate) == 0 {
		return nil, errors.New("certificate/key unreadable or mismatched")
	}
	leaf, e := x509.ParseCertificate(pair.Certificate[0])
	if e != nil {
		return nil, errors.New("invalid certificate")
	}
	if domain != "" {
		if e = leaf.VerifyHostname(domain); e != nil {
			return nil, errors.New("certificate does not match the configured domain")
		}
	}
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return nil, errors.New("certificate is expired or not yet valid")
	}
	return leaf, nil
}

func tunnelCertificate(t config.TunnelConfig) (string, string, bool) {
	if t.Transport == config.TransportNaive {
		return certs.FindCaddy(t.Domain)
	}
	if t.Transport == config.TransportSlipstream && t.Slipstream != nil {
		return t.Slipstream.Cert, t.Slipstream.Key, true
	}
	return "", "", false
}

func handleSystemCertificates(ctx *actions.Context) error {
	cfg := ctx.Config.(*config.Config)
	out := ctx.Output
	mode := ctx.GetArg("mode")
	if mode == "" {
		mode = "check"
	}
	if mode == "check" {
		var problems []error
		for _, t := range cfg.Tunnels {
			if t.Transport != config.TransportNaive && t.Transport != config.TransportSlipstream {
				continue
			}
			crt, key, ok := tunnelCertificate(t)
			if !ok {
				out.Error(t.Tag + ": certificate missing")
				problems = append(problems, fmt.Errorf("%s: certificate missing", t.Tag))
				continue
			}
			domain := t.Domain
			if t.Transport == config.TransportSlipstream {
				domain = ""
			} // legacy pin-only certificates may use the tunnel tag as their name
			leaf, e := certificatePair(crt, key, domain, time.Now())
			if e != nil {
				out.Error(t.Tag + ": " + e.Error())
				problems = append(problems, fmt.Errorf("%s: %w", t.Tag, e))
				continue
			}
			out.Success(fmt.Sprintf("%s: valid until %s", t.Tag, leaf.NotAfter.UTC().Format(time.RFC3339)))
		}
		return errors.Join(problems...)
	}
	if mode != "recover" {
		return errors.New("certificate mode must be check or recover")
	}
	tag := ctx.GetArg("tag")
	if tag == "" {
		var opts []actions.SelectOption
		for _, t := range cfg.Tunnels {
			if t.Transport == config.TransportNaive || t.Transport == config.TransportSlipstream {
				opts = append(opts, actions.SelectOption{Value: t.Tag, Label: t.Tag + " · " + t.Domain})
			}
		}
		if len(opts) == 0 {
			return errors.New("no TLS front or Slipstream tunnel configured")
		}
		var e error
		tag, e = prompt.Select("Certificate to recover", opts)
		if e != nil {
			return e
		}
	}
	ptr := cfg.GetTunnel(tag)
	if ptr == nil {
		return errors.New("tunnel not found")
	}
	t := *ptr
	if t.Transport == config.TransportNaive {
		if state, _ := service.Status(service.TunnelServiceName(tag)); state != "active" {
			return errors.New("TLS front is stopped; start it before ACME recovery")
		}
		out.Info("Caddy manages ACME renewal. Reopening the TLS front; existing sessions may reconnect.")
		if e := service.Restart(service.TunnelServiceName(tag)); e != nil {
			return e
		}
		deadline := time.Now().Add(90 * time.Second)
		for {
			crt, key, found := certs.FindCaddy(t.Domain)
			if found && validSetupCertificate(crt, key, t.Domain) == nil {
				restarted, e := coreinstall.RestartIfCertChanged()
				if e != nil {
					return e
				}
				out.Success(fmt.Sprintf("Valid ACME certificate available; refreshed %d dependent cores", len(restarted)))
				return nil
			}
			if time.Now().After(deadline) {
				return errors.New("ACME recovery pending: check public A/AAAA records, ports 80/443 and issuer reachability; existing certificate files retained")
			}
			time.Sleep(time.Second)
		}
	}
	if t.Transport != config.TransportSlipstream || t.Slipstream == nil {
		return errors.New("selected tunnel does not manage a certificate")
	}
	if _, e := certificatePair(t.Slipstream.Cert, t.Slipstream.Key, "", time.Now()); e != nil {
		out.Info("Replacing an invalid or expired Slipstream certificate changes its public pin. Clients must refresh/import the new public certificate.")
		dir, e := os.MkdirTemp(config.TunnelDir(t.Tag), "cert-recovery-")
		if e != nil {
			return e
		}
		crt, key := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
		if e = certs.GenerateSelfSigned(crt, key, t.Domain); e != nil {
			return e
		}
		if _, e = certificatePair(crt, key, t.Domain, time.Now()); e != nil {
			return e
		}
		// Both complete files exist before a single atomic configuration publication.
		// Keep old paths/files for recovery; never truncate the live pair in place.
		next := *t.Slipstream
		next.Cert, next.Key = crt, key
		t.Slipstream = &next
		previous := *ptr
		cfg.UpdateTunnel(t)
		if e = cfg.Save(); e != nil {
			cfg.UpdateTunnel(previous)
			return e
		}
	}
	// Also run on a retry with an already-valid new pair, converging units and
	// the DNS verification fingerprint after an interrupted recovery.
	if e := transport.RegenerateService(&t, cfg); e != nil {
		return e
	}
	if e := dnsrouter.RegenerateRouterService(); e != nil {
		return e
	}
	if e := WriteSubscriptions(cfg, out); e != nil {
		return e
	}
	out.Success("Slipstream certificate ready; public certificate companion refreshed; previous certificate files retained")
	return nil
}
