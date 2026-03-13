# Cloudflare Tunnel ingress configuration
# Manages all routing rules for forumline.net services.
# Changes here go through PR review → terraform plan → terraform apply.
# No more SSH + config edit + restart.
#
# IMPORTANT: Rule order matters! Cloudflare evaluates top-to-bottom.
# Specific hostnames MUST come before wildcards, or they'll never match.

resource "cloudflare_zero_trust_tunnel_cloudflared_config" "forumline" {
  account_id = var.cloudflare_account_id
  tunnel_id  = var.tunnel_id

  config {
    # Website — forumline.net
    ingress_rule {
      hostname = "forumline.net"
      service  = "http://192.168.1.106:3000"
    }

    # Demo Forum — demo.forumline.net (localhost because cloudflared runs on this LXC)
    ingress_rule {
      hostname = "demo.forumline.net"
      service  = "http://localhost:3000"
    }

    # Forumline App — app.forumline.net
    ingress_rule {
      hostname = "app.forumline.net"
      service  = "http://192.168.1.99:3000"
    }

    # Hosted Platform API — hosted.forumline.net
    ingress_rule {
      hostname = "hosted.forumline.net"
      service  = "http://192.168.1.107:3000"
    }

    # Dozzle Log Viewer — logs.forumline.net
    ingress_rule {
      hostname = "logs.forumline.net"
      service  = "http://192.168.1.108:8080"
    }

    # SSH access for deploys (MUST be before *.forumline.net wildcard)
    ingress_rule {
      hostname = "ssh.forumline.net"
      service  = "ssh://localhost:22"
    }

    ingress_rule {
      hostname = "app-ssh.forumline.net"
      service  = "ssh://192.168.1.99:22"
    }

    ingress_rule {
      hostname = "www-ssh.forumline.net"
      service  = "ssh://192.168.1.106:22"
    }

    ingress_rule {
      hostname = "hosted-ssh.forumline.net"
      service  = "ssh://192.168.1.107:22"
    }

    # Logs SSH — logs-ssh.forumline.net
    ingress_rule {
      hostname = "logs-ssh.forumline.net"
      service  = "ssh://192.168.1.108:22"
    }

    # Hosted Forum Tenants — *.forumline.net wildcard (MUST be last before catch-all)
    ingress_rule {
      hostname = "*.forumline.net"
      service  = "http://192.168.1.107:3000"
    }

    # Catch-all (required by Cloudflare)
    ingress_rule {
      service = "http_status:404"
    }
  }

  lifecycle {
    prevent_destroy = true
  }
}
