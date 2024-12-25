
# UbiqAI


**UbiqAI** is the free, Open Source OpenAI alternative. UbiqAI act as a drop-in replacement REST API that’s compatible with OpenAI (Elevenlabs, Anthropic... ) API specifications for local AI inferencing. It allows you to run LLMs, generate images, audio (and not only) locally or on-prem with consumer grade hardware, supporting multiple model families. Does not require GPU. It is created and maintained by [Ettore Di Giacinto](https://github.com/mudler).

Run the installer script:

```bash
curl https://ubiqai.io/install.sh | sh
```

Or run with docker:
```bash
# CPU only image:
docker run -ti --name ubiq-ai -p 8080:8080 ubiqai/ubiqai:latest-cpu

# Nvidia GPU:
docker run -ti --name ubiq-ai -p 8080:8080 --gpus all ubiqai/ubiqai:latest-gpu-nvidia-cuda-12

# CPU and GPU image (bigger size):
docker run -ti --name ubiq-ai -p 8080:8080 ubiqai/ubiqai:latest

# AIO images (it will pre-download a set of models ready for use, see https://ubiqai.io/basics/container/)
docker run -ti --name ubiq-ai -p 8080:8080 ubiqai/ubiqai:latest-aio-cpu
```

To load models:

```bash
# From the model gallery (see available models with `ubiq-ai models list`, in the WebUI from the model tab, or visiting https://models.ubiqai.io)
ubiq-ai run llama-3.2-1b-instruct:q4_k_m
# Start ubiqai with the phi-2 model directly from huggingface
ubiq-ai run huggingface://TheBloke/phi-2-GGUF/phi-2.Q8_0.gguf
# Install and run a model from the Ollama OCI registry
ubiq-ai run ollama://gemma:2b
# Run a model from a configuration file
ubiq-ai run https://gist.githubusercontent.com/.../phi-2.yaml
# Install and run a model from a standard OCI registry (e.g., Docker Hub)
ubiq-ai run oci://ubiqai/phi-2:latest
```

