package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	veloxquant "github.com/rajveer43/veloxquant-go"
)

func runModels(args []string) error {
	fs := flag.NewFlagSet("models", flag.ContinueOnError)
	local := fs.Bool("local", false, "List downloaded/cached models and disk usage")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}

	client, err := veloxquant.NewClient()
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	ctx := context.Background()

	if *local {
		localModels, err := client.Models.Local(ctx)
		if err != nil {
			return fmt.Errorf("scan local models: %w", err)
		}

		if len(localModels) == 0 {
			fmt.Println("No cached models found locally.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "MODEL\tSIZE\tLAST MODIFIED\tPATH")

		var totalSize uint64
		for _, m := range localModels {
			totalSize += m.SizeBytes
			modTime := "-"
			if m.LastModified != nil {
				modTime = m.LastModified.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				m.Name,
				veloxquant.FormatBytes(m.SizeBytes),
				modTime,
				m.Path,
			)
		}
		w.Flush()

		countUnit := "models"
		if len(localModels) == 1 {
			countUnit = "model"
		}

		fmt.Printf("\nTotal cached model footprint: %s (%d %s)\n",
			veloxquant.FormatBytes(totalSize),
			len(localModels),
			countUnit,
		)
		return nil
	}

	all := client.Models.List()
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "MODEL\tPARAMETERS\tRECOMMENDED")
	for _, m := range all {
		rec := "-"
		if m.Recommended {
			rec = "yes"
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n", m.Name, m.Parameters, rec)
	}
	w.Flush()

	return nil
}
