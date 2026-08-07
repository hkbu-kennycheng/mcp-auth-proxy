package main

import (
	"os"
	"strings"

	mcpproxy "github.com/sigbit/mcp-auth-proxy/v2/pkg/mcp-proxy"
	"github.com/spf13/cobra"
)

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBoolWithDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return strings.EqualFold(value, "true") || value == "1"
	}
	return defaultValue
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func splitWithEscapes(s, delimiter string) []string {
	if s == "" {
		return []string{}
	}
	var result []string
	var current strings.Builder
	escaped := false
	for i := 0; i < len(s); i++ {
		if escaped {
			current.WriteByte(s[i])
			escaped = false
		} else if s[i] == '\\' && i+1 < len(s) {
			if strings.HasPrefix(s[i+1:], delimiter) {
				escaped = true
			} else {
				current.WriteByte(s[i])
			}
		} else if strings.HasPrefix(s[i:], delimiter) {
			result = append(result, strings.TrimSpace(current.String()))
			current.Reset()
			i += len(delimiter) - 1
		} else {
			current.WriteByte(s[i])
		}
	}
	result = append(result, strings.TrimSpace(current.String()))
	return result
}

func parseAttributeMap(s string) map[string][]string {
	result := make(map[string][]string)
	if s == "" {
		return result
	}
	parts := splitWithEscapes(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		eqIdx := strings.Index(part, "=")
		if eqIdx == -1 {
			continue
		}
		key := strings.TrimSpace(part[:eqIdx])
		value := strings.TrimSpace(part[eqIdx+1:])
		if key != "" && value != "" {
			result[key] = append(result[key], value)
		}
	}
	return result
}

func parseHeaderMapping(s string) map[string]string {
	result := make(map[string]string)
	if s == "" {
		return result
	}
	parts := splitWithEscapes(s, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		colonIdx := strings.LastIndex(part, ":")
		if colonIdx == -1 {
			continue
		}
		pointer := strings.TrimSpace(part[:colonIdx])
		header := strings.TrimSpace(part[colonIdx+1:])
		if pointer != "" && header != "" {
			result[pointer] = header
		}
	}
	return result
}

func main() {
	if err := newRootCommand(mcpproxy.Run).Execute(); err != nil {
		panic(err)
	}
}

func newRootCommand(run mcpproxy.ProxyRunnerFunc) *cobra.Command {
	var listen string
	var externalURL string

	rootCmd := &cobra.Command{
		Use: "mcp-auth-proxy",
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(listen, externalURL, args); err != nil {
				panic(err)
			}
		},
	}

	rootCmd.Flags().StringVar(&listen, "listen", getEnvWithDefault("LISTEN", ":80"), "Address to listen on")
	rootCmd.Flags().StringVarP(&externalURL, "external-url", "e", getEnvWithDefault("EXTERNAL_URL", "http://localhost"), "External URL for the proxy")

	return rootCmd
}
