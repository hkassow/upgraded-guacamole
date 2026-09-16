## WIP 

This is the ai server running ollama + a simple go server to redirect traffic to the model





# text only
ollama pull qwen3:14b
ollama pull qwen2.5vl:7b

ollama create recipe-parser -f ./Modelfile.parser
ollama create recipe-vision -f ./Modelfile.vision

ollama run recipe-parser

# image parsing
ollama create recipe-vision -f ./Modelfile.vision