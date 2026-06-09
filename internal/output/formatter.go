package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

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
		return "", fmt.Errorf("unknown output format %q (valid: table, json, yaml)", s)
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

func (f *Formatter) PrintYAML(v any) error {
	enc := yaml.NewEncoder(f.Writer)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(v)
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
