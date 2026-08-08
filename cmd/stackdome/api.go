package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/Stackdome/stackdome-cli/internal/client"
	"github.com/Stackdome/stackdome-cli/internal/cmdutil"
	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/Stackdome/stackdome-cli/internal/output"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newAPICmd() *cobra.Command {
	var method string
	var data string
	var dataFile string
	var headerFlags []string
	var assumeYes bool

	cmd := &cobra.Command{
		Use:   "api PATH",
		Short: "Send an authenticated request to a Stackdome API path",
		Args:  cobra.ExactArgs(1),
		RunE: cmdutil.WithContext(func(ctx *cmdutil.CommandContext, cmd *cobra.Command, args []string) error {
			if err := ctx.Config.RequireAuth(); err != nil {
				return err
			}
			if err := validateAPIPath(args[0]); err != nil {
				return err
			}

			method = strings.ToUpper(method)
			if !isAPIMethod(method) {
				return clierrors.ValidationError(fmt.Sprintf("unsupported HTTP method %q", method))
			}
			if cmd.Flags().Changed("data") && cmd.Flags().Changed("data-file") {
				return clierrors.ValidationError("--data and --data-file cannot be used together")
			}

			headers, err := parseAPIHeaders(headerFlags)
			if err != nil {
				return err
			}
			if headers.Get("Accept") == "" {
				headers.Set("Accept", "application/json")
			}
			if headers.Get("Content-Type") == "" {
				headers.Set("Content-Type", "application/json")
			}

			body := []byte(data)
			if dataFile != "" {
				body, err = os.ReadFile(dataFile)
				if err != nil {
					return clierrors.Wrapf(err, "read request body from %s", dataFile)
				}
			}

			if isMutatingAPIMethod(method) {
				if _, err := cmdutil.Confirm(ctx.Formatter, fmt.Sprintf("Send %s request to %s?", method, args[0]), assumeYes); err != nil {
					return err
				}
			}

			response, err := ctx.Client.APIRequest(cmd.Context(), method, args[0], headers, body)
			if err != nil {
				return client.WrapError(nil, err, "API request failed")
			}
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				return client.WrapError(&http.Response{StatusCode: response.StatusCode, Header: response.Header}, errors.New(safeAPIErrorReason(response.Body, ctx.Config.AccessToken, ctx.Config.RefreshToken)), "API request failed")
			}
			return writeAPIResponse(ctx.Formatter, response.Body)
		}),
	}

	cmd.Flags().StringVarP(&method, "method", "X", http.MethodGet, "HTTP method (GET, HEAD, POST, PUT, PATCH, DELETE)")
	cmd.Flags().StringVar(&data, "data", "", "JSON request body")
	cmd.Flags().StringVar(&dataFile, "data-file", "", "Path to a JSON request body")
	cmd.Flags().StringArrayVarP(&headerFlags, "header", "H", nil, "Request header (Name: value; repeatable)")
	cmd.Flags().BoolVar(&assumeYes, "yes", false, "Skip confirmation for mutating requests")
	return cmd
}

func validateAPIPath(path string) error {
	if err := client.ValidateAPIPath(path); err != nil {
		return clierrors.ValidationError("PATH must be a relative path beginning with /api/ and without a fragment")
	}
	return nil
}

func isAPIMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isMutatingAPIMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func parseAPIHeaders(values []string) (http.Header, error) {
	headers := make(http.Header)
	for _, value := range values {
		name, headerValue, ok := strings.Cut(value, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, clierrors.ValidationError(fmt.Sprintf("invalid header %q; use Name: value", value))
		}
		if isProtectedAPIHeader(name) {
			return nil, clierrors.ValidationError(fmt.Sprintf("header %q cannot be overridden", name))
		}
		headers.Add(name, strings.TrimSpace(headerValue))
	}
	return headers, nil
}

func isProtectedAPIHeader(name string) bool {
	for _, protected := range []string{"Authorization", "Proxy-Authorization", "Host", "Cookie"} {
		if strings.EqualFold(name, protected) {
			return true
		}
	}
	return false
}

func writeAPIResponse(formatter *output.Formatter, body []byte) error {
	if len(body) == 0 {
		return nil
	}
	if formatter.Format != output.FormatYAML {
		_, err := formatter.Writer.Write(body)
		return err
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var response any
	if err := decoder.Decode(&response); err != nil {
		return clierrors.Wrap(err, "response is not valid JSON for YAML output")
	}
	if err := requireJSONEOF(decoder); err != nil {
		return clierrors.Wrap(err, "response is not valid JSON for YAML output")
	}
	node, err := jsonYAMLNode(response)
	if err != nil {
		return err
	}
	encoder := yaml.NewEncoder(formatter.Writer)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(node)
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func jsonYAMLNode(value any) (*yaml.Node, error) {
	switch value := value.(type) {
	case map[string]any:
		node := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			child := value[key]
			childNode, err := jsonYAMLNode(child)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, childNode)
		}
		return node, nil
	case []any:
		node := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		for _, child := range value {
			childNode, err := jsonYAMLNode(child)
			if err != nil {
				return nil, err
			}
			node.Content = append(node.Content, childNode)
		}
		return node, nil
	case json.Number:
		tag := "!!int"
		if strings.ContainsAny(value.String(), ".eE") {
			tag = "!!float"
		}
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value.String()}, nil
	case string:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}, nil
	case bool:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: fmt.Sprint(value)}, nil
	case nil:
		return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
	default:
		return nil, clierrors.New("response contains unsupported JSON value")
	}
}

func safeAPIErrorReason(body []byte, secrets ...string) string {
	var response struct {
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(body, &response); err != nil || response.Reason == "" {
		return "API request failed"
	}
	return redactSecrets(response.Reason, secrets...)
}
