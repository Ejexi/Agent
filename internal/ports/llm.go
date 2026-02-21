package ports

import "context"

type LLM interface {

	Name() string

	Generate(
		ctx context.Context,
		prompt string,
	) (string, error)
}

type LLMRegistry interface {

	Register(llm LLM)

	Get(name string) LLM

	List() []string
}