# LLM Gate Proxy Service

## Introduction

LLM Gate is an open-source proxy service designed to facilitate the use of various language learning models (LLMs) including OpenAI, Gemini, and a mock service for testing. The service accepts requests in the OpenAI format and allows seamless integration and switching between different LLM providers. It's particularly useful for developers looking to integrate LLM features without directly hitting provider APIs during development and testing phases.

## Features

- **Multiple Providers:** Easily switch between OpenAI, Gemini, and a mock service.
- **Mock Service:** Includes a mock service that simulates LLM responses, helping reduce development costs and bypass the need for API keys during initial testing.
- **Open Source:** Free to use and modify, making it ideal for community-driven enhancements or personal projects.

## Usage

### Making Requests
To make requests to the LLM Gate, you need to specify the provider and send a JSON payload in the OpenAI format.

#### Endpoint
```bash
POST https://llmgate.uc.r.appspot.com/completions?provider=Mock
```

#### Header
```bash
Content-Type: application/json
key: <Your-OpenAI-Key/Gemini-Key/Any-String-For-Mock>
```

#### Request Payload Example
```bash
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": [{
      	"type": "text",
      	"text": "hi"
      }
      ]
    }
  ],
  "temperature": 0.4,
  "max_tokens": 1000
}
```

#### Response Example
```bash
{
    "id": "chatcmpl-9ls0a7SKT5l1ew62hpYdtYRMesBON",
    "object": "chat.completion",
    "created": 1721196052,
    "model": "gpt-4o-2024-05-13",
    "choices": [
        {
            "index": 0,
            "message": {
                "role": "assistant",
                "content": "Hello! How can I assist you today?"
            },
            "finish_reason": "stop"
        }
    ],
    "usage": {
        "prompt_tokens": 8,
        "completion_tokens": 9,
        "total_tokens": 17
    }
}
```

## Running Locally

### Prerequisites

- Go 1.16+ for building and running the service.
- Internet access is required for reaching OpenAI or Gemini APIs, but not necessary for using the mock service.

### Installation

To get started with the LLM Gate, clone the repository and build the service:

```bash
git clone https://github.com/llmgate/llmgate.git
cd llmgate
go build
```

### Running the Service
To start the service locally, run:
```bash
./llmgate
```
The service will be available at `http://localhost:8080`.

## Contributing
Contributions are what make the open-source community such an amazing place to learn, inspire, and create. Any contributions you make are greatly appreciated.

Check out our contributing.md for more information on how to start contributing.