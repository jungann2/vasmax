package nginx

import (
	"fmt"
	"strings"
)

// generateServerBlock generates the main Nginx server block configuration.
func generateServerBlock(params *NginxParams) string {
	var b strings.Builder

	// HTTP → HTTPS redirect
	b.WriteString("server {\n")
	b.WriteString("    listen 80;\n")
	b.WriteString("    listen [::]:80;\n")
	b.WriteString(fmt.Sprintf("    server_name %s;\n", params.Domain))
	b.WriteString("    return 301 https://$server_name$request_uri;\n")
	b.WriteString("}\n\n")

	// HTTPS server block
	b.WriteString("server {\n")
	b.WriteString("    listen 443 ssl http2;\n")
	b.WriteString("    listen [::]:443 ssl http2;\n")
	b.WriteString(fmt.Sprintf("    server_name %s;\n\n", params.Domain))

	// TLS settings
	b.WriteString(fmt.Sprintf("    ssl_certificate %s;\n", params.CertFile))
	b.WriteString(fmt.Sprintf("    ssl_certificate_key %s;\n", params.KeyFile))
	b.WriteString("    ssl_protocols TLSv1.2 TLSv1.3;\n")
	b.WriteString("    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES256-GCM-SHA384;\n")
	b.WriteString("    ssl_prefer_server_ciphers on;\n")
	b.WriteString("    ssl_session_cache shared:SSL:10m;\n")
	b.WriteString("    ssl_session_timeout 10m;\n\n")

	// Default root
	b.WriteString(fmt.Sprintf("    root %s;\n", DefaultHTMLDir))
	b.WriteString("    index index.html;\n\n")

	// Protocol location blocks
	for _, p := range params.Protocols {
		b.WriteString(generateLocationBlock(p.Type, p.Path, p.BackendPort))
		b.WriteString("\n")
	}

	b.WriteString("    # --- END LOCATIONS ---\n\n")

	// Default location
	b.WriteString("    location / {\n")
	b.WriteString("        try_files $uri $uri/ =404;\n")
	b.WriteString("    }\n")
	b.WriteString("}\n")

	return b.String()
}

// generateLocationBlock generates a location block for a specific protocol.
func generateLocationBlock(protocolType, path string, backendPort int) string {
	var b strings.Builder
	tag := strings.ToUpper(strings.ReplaceAll(protocolType, "/", "_"))

	b.WriteString(fmt.Sprintf("    # --- BEGIN %s ---\n", tag))

	switch protocolType {
	case "ws":
		b.WriteString(fmt.Sprintf("    location %s {\n", path))
		b.WriteString(fmt.Sprintf("        proxy_pass http://127.0.0.1:%d;\n", backendPort))
		b.WriteString("        proxy_http_version 1.1;\n")
		b.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
		b.WriteString("        proxy_set_header Connection \"upgrade\";\n")
		b.WriteString("        proxy_set_header Host $host;\n")
		b.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
		b.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		b.WriteString("        proxy_read_timeout 300s;\n")
		b.WriteString("        proxy_send_timeout 300s;\n")
		b.WriteString("    }\n")

	case "grpc":
		b.WriteString(fmt.Sprintf("    location ^~ /%s {\n", path))
		b.WriteString(fmt.Sprintf("        grpc_pass grpc://127.0.0.1:%d;\n", backendPort))
		b.WriteString("        grpc_set_header Host $host;\n")
		b.WriteString("        grpc_set_header X-Real-IP $remote_addr;\n")
		b.WriteString("        grpc_read_timeout 300s;\n")
		b.WriteString("        grpc_send_timeout 300s;\n")
		b.WriteString("    }\n")

	case "httpupgrade":
		b.WriteString(fmt.Sprintf("    location %s {\n", path))
		b.WriteString(fmt.Sprintf("        proxy_pass http://127.0.0.1:%d;\n", backendPort))
		b.WriteString("        proxy_http_version 1.1;\n")
		b.WriteString("        proxy_set_header Upgrade $http_upgrade;\n")
		b.WriteString("        proxy_set_header Connection \"upgrade\";\n")
		b.WriteString("        proxy_set_header Host $host;\n")
		b.WriteString("        proxy_set_header X-Real-IP $remote_addr;\n")
		b.WriteString("        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;\n")
		b.WriteString("    }\n")

	default:
		// Generic TCP proxy location
		b.WriteString(fmt.Sprintf("    location %s {\n", path))
		b.WriteString(fmt.Sprintf("        proxy_pass http://127.0.0.1:%d;\n", backendPort))
		b.WriteString("        proxy_set_header Host $host;\n")
		b.WriteString("    }\n")
	}

	b.WriteString(fmt.Sprintf("    # --- END %s ---\n", tag))
	return b.String()
}

// generateSubscribeLocation generates the subscription server location block.
func generateSubscribeLocation() string {
	var b strings.Builder
	b.WriteString("    # --- BEGIN SUBSCRIBE ---\n")
	b.WriteString("    location /s/ {\n")
	b.WriteString("        alias /etc/vasmax/subscribe/;\n")
	b.WriteString("        default_type 'text/plain; charset=utf-8';\n")
	b.WriteString("        add_header Cache-Control no-cache;\n")
	b.WriteString("    }\n")
	b.WriteString("    # --- END SUBSCRIBE ---\n")
	return b.String()
}
