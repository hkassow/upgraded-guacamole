## WIP 

This is the ai server running ollama + a simple go server to redirect traffic to the model





# model startup
ollama pull qwen3:14b
ollama pull qwen2.5vl:7b

ollama create recipe-parser -f ./Modelfile.parser
ollama create recipe-vision -f ./Modelfile.vision


ollama list
ollama ps

# start server
go run server.go


# run model independent of server
ollama run recipe-parser


# test text model
curl -X POST http://localhost:8556/parse-recipe \
  -H "X-API-Key: test-api-key" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "2 eggs, beaten\n1 cup flour\n\nMix eggs and flour together."}'


# test vision model
base64 -w 0 choco-cake.jpg > image_b64.txt

python3 -c "
import json
with open('image_b64.txt') as f:
    img = f.read().strip()
print(json.dumps({'image': img}))
" > payload.json

curl -X POST http://localhost:8556/parse-recipe-image \
  -H "X-API-Key: test-api-key" \
  -H "Content-Type: application/json" \
  -d @payload.json
