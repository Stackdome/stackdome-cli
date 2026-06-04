package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

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

func (f *Formatter) NewTable() *Table {
	tw := tabwriter.NewWriter(f.Writer, 0, 0, 2, ' ', 0)
	return &Table{tw: tw}
}

type Table struct {
	tw *tabwriter.Writer
}

func (t *Table) AddHeader(cols ...string) {
	for i, col := range cols {
		if i > 0 {
			fmt.Fprint(t.tw, "\t")
		}
		fmt.Fprint(t.tw, Bold(col))
	}
	fmt.Fprintln(t.tw)
}

func (t *Table) AddRow(cols ...string) {
	for i, col := range cols {
		if i > 0 {
			fmt.Fprint(t.tw, "\t")
		}
		fmt.Fprint(t.tw, col)
	}
	fmt.Fprintln(t.tw)
}

func (t *Table) Flush() error {
	return t.tw.Flush()
}

func (f *Formatter) Println(args ...any) {
	fmt.Fprintln(f.Writer, args...)
}

func (f *Formatter) Printf(format string, args ...any) {
	fmt.Fprintf(f.Writer, format, args...)
}
