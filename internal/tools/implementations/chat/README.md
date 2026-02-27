# tools/implementations/chat/

Chat tool — LLM conversation via function calling.

## Purpose

Sends user prompts to an LLM provider and returns AI-generated responses. Uses the `LLMRegistry` to select the active provider.

## Registration

Registered in bootstrap as: `chat.NewChatTool(deps.LLM)`
