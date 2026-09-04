module github.com/rajveer43/veloxquant-go/examples/langchain

go 1.26.2

require (
	github.com/rajveer43/veloxquant-go v0.0.0
	github.com/rajveer43/veloxquant-go/langchain v0.0.0
	github.com/tmc/langchaingo v0.1.14
)

require (
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pkoukk/tiktoken-go v0.1.6 // indirect
)

replace github.com/rajveer43/veloxquant-go => ../../

replace github.com/rajveer43/veloxquant-go/langchain => ../../langchain
