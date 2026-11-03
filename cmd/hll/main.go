// Command hll estimates distinct counts from line-oriented input.
//
// Usage:
//
//	hll add [-p precision] [-in file] [-out sketchfile]
//	    Read one value per line and print the estimated distinct count.
//	    With -out, also save the sketch for later merging.
//
//	hll merge -out merged.sketch sketch1 sketch2 ...
//	    Merge sketches into one and print the union's estimated count.
//
//	hll count sketchfile
//	    Print the estimated distinct count stored in a sketch file.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/Subhanmalik138/go-hyperloglog/hll"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "add":
		err = runAdd(os.Args[2:])
	case "merge":
		err = runMerge(os.Args[2:])
	case "count":
		err = runCount(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hll:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: hll <add|merge|count> [flags] [args]")
	fmt.Fprintln(os.Stderr, "  hll add [-p 14] [-in file] [-out sketch]")
	fmt.Fprintln(os.Stderr, "  hll merge -out merged.sketch sketch1 sketch2 ...")
	fmt.Fprintln(os.Stderr, "  hll count sketch")
}

func runAdd(args []string) error {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	precision := fs.Int("p", hll.DefaultPrecision, "sketch precision (4-18)")
	inPath := fs.String("in", "", "input file (default: stdin)")
	outPath := fs.String("out", "", "sketch file to save")
	if err := fs.Parse(args); err != nil {
		return err
	}
	sketch, err := hll.New(*precision)
	if err != nil {
		return err
	}
	in := os.Stdin
	if *inPath != "" {
		f, err := os.Open(*inPath)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		sketch.AddString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if *outPath != "" {
		data, err := sketch.MarshalBinary()
		if err != nil {
			return err
		}
		if err := os.WriteFile(*outPath, data, 0o644); err != nil {
			return err
		}
	}
	fmt.Println(sketch.Count())
	return nil
}

func runMerge(args []string) error {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	outPath := fs.String("out", "", "merged sketch file to write")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *outPath == "" {
		return fmt.Errorf("merge requires -out")
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("merge requires at least one sketch file")
	}
	var merged *hll.Sketch
	for _, path := range fs.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sk, err := hll.UnmarshalBinary(data)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if merged == nil {
			merged = sk
		} else if err := merged.Merge(sk); err != nil {
			return err
		}
	}
	data, err := merged.MarshalBinary()
	if err != nil {
		return err
	}
	if err := os.WriteFile(*outPath, data, 0o644); err != nil {
		return err
	}
	fmt.Println(merged.Count())
	return nil
}

func runCount(args []string) error {
	fs := flag.NewFlagSet("count", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("count requires exactly one sketch file")
	}
	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	sk, err := hll.UnmarshalBinary(data)
	if err != nil {
		return err
	}
	fmt.Println(sk.Count())
	return nil
}
