package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {
	var (
		serviceName = flag.String("service", "", "Service name to generate")
		outputDir   = flag.String("output", "./services", "Output directory for generated services")
	)
	flag.Parse()

	if *serviceName == "" {
		fmt.Println("Usage: go run . -service <service_name> [-output <output_directory>]")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  go run . -service customer -output ./generated")
		fmt.Println("  go run . -service ticket -output ./tmp/generated")
		return
	}

	// Construct DSL path from service name
	dslPath := fmt.Sprintf("./dsl/apps/%s/service.json", *serviceName)

	generator := NewServiceGenerator()
	if err := generator.GenerateService(dslPath, *outputDir); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Service generation completed successfully!")
}
