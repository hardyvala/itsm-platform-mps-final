package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	var (
		dslPath   = flag.String("dsl", "", "Path to DSL file")
		outputDir = flag.String("output", "./services", "Output directory for generated services")
	)
	flag.Parse()

	if *dslPath == "" {
		fmt.Println("Usage: go run . -dsl <path_to_dsl_file> [-output <output_directory>]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  go run . -dsl ./services/ticket-service/dsl/service.json")
		fmt.Println("  go run . -dsl ./my-service.json -output ./generated")
		return
	}

	generator := NewServiceGenerator()
	if err := generator.GenerateService(*dslPath, *outputDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Service generation completed successfully!")
}
