
func AIQuery() {
	ctx := context.Background()
	// Initialize Genkit with xAI
	g := genkit.Init(ctx,
		genkit.WithPlugins(&compat_oai.OpenAICompatible{
			Provider: "xai",
			APIKey:   os.Getenv("XAI_API_KEY"),
			BaseURL:  "https://api.x.ai/v1",
		}),
		genkit.WithDefaultModel("xai/grok-3"),
	)
	// Define the flow and capture the returned Flow object
	grokHelloFlow := genkit.DefineFlow(g, "grokHello",
		func(ctx context.Context, subject string) (string, error) {
			resp, err := genkit.Generate(ctx, g,
				ai.WithModelName("xai/grok-3"),
				ai.WithPrompt(fmt.Sprintf("Tell me a fun fact about %s.", subject)),
			)
			if err != nil {
				return "", err
			}
			return resp.Text(), nil
		},
	)
	// Run the flow using the .Run() method on the flow object
	result, err := grokHelloFlow.Run(ctx, "Grok and xAI")
	if err != nil {
		log.Fatalf("Error running flow: %v", err)
	}
	fmt.Println("Response from Grok:")
	fmt.Println(result)
}

AITaskManager(){



}
