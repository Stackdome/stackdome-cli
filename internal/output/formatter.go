package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	clierrors "github.com/Stackdome/stackdome-cli/internal/errors"
	"github.com/charmbracelet/lipgloss"
	lgTable "github.com/charmbracelet/lipgloss/table"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

func ParseFormat(s string) (Format, error) {
	switch s {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "yaml":
		return FormatYAML, nil
	default:
		return "", clierrors.ValidationError(fmt.Sprintf("unknown output format %q (valid: table, json, yaml)", s))
	}
}

type Formatter struct {
	Format Format
	Writer io.Writer
}

func NewFormatter(format Format) *Formatter {
	return &Formatter{
		Format: format,
		Writer: os.Stdout,
	}
}

func (f *Formatter) PrintJSON(v any) error {
	enc := json.NewEncoder(f.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// PrintJSONLine writes one compact JSON value followed by a newline, suitable
// for NDJSON streams where each event must be independently decodable.
func (f *Formatter) PrintJSONLine(v any) error {
	return json.NewEncoder(f.Writer).Encode(v)
}

// PrintYAML routes through JSON first so the API types' `json:` tags decide the
// key names — yaml.Marshal alone would emit lowercased Go field names.
func (f *Formatter) PrintYAML(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var generic any
	if err := json.Unmarshal(b, &generic); err != nil {
		return err
	}

	enc := yaml.NewEncoder(f.Writer)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(generic)
}

func (f *Formatter) PrintStructured(v any) error {
	switch f.Format {
	case FormatJSON:
		return f.PrintJSON(v)
	case FormatYAML:
		return f.PrintYAML(v)
	default:
		return nil
	}
}

func (f *Formatter) IsTable() bool {
	return f.Format == FormatTable
}

func ValidateStreamingFormat(format Format) error {
	if format == FormatYAML {
		return clierrors.ValidationError("YAML output is not supported for streaming commands; use -o json or table")
	}
	return nil
}

func (f *Formatter) NewTable(headers ...string) *Table {
	return &Table{
		writer:  f.Writer,
		headers: headers,
	}
}

type Table struct {
	writer  io.Writer
	headers []string
	rows    [][]string
}

func (t *Table) AddRow(cols ...string) {
	t.rows = append(t.rows, cols)
}

func (t *Table) Render() {
	tbl := lgTable.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).BorderHeader(false).
		Headers(t.headers...).
		Rows(t.rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == lgTable.HeaderRow {
				return lipgloss.NewStyle().Bold(true).PaddingRight(2)
			}
			return lipgloss.NewStyle().PaddingRight(2)
		})

	fmt.Fprintln(t.writer, tbl)
}

func (f *Formatter) Println(args ...any) {
	fmt.Fprintln(f.Writer, args...)
}

func (f *Formatter) Printf(format string, args ...any) {
	fmt.Fprintf(f.Writer, format, args...)
}
