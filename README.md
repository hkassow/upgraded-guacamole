## upgraded-guacamole
A recipe parser for storing recipes, uses vanilla go backend and vanilla javascript frontend. 
Currently uses deepinfra api with Qwen models to handle image and text parsing

# Run locally using docker
docker compose up --build

docker compose down

`127.0.0.1:8443`

# Project layout
```
main.go                  routes, starts the db, migrations and recipe worker
handlers/                HTTP handlers (recipes, ingredients, login/sessions, follows)
lib/
  recipe-service.go      recipes: create, read, update, delete
  recipe-jobs.go         recipe_jobs rows (recipes waiting to be parsed)
  recipe-worker.go       queue + worker that parses jobs and saves the recipe
  recipe-parser.go       turning the model's JSON into a recipe and cleaning it up
  recipe-prompts.go      the prompts sent to the models
  deepinfra.go           DeepInfra client (streamed chat completions, photo transcription)
  self-hosted-parser.go  client for the old gouda-woulda model server (not used right now)
  tags.go                recipe tags
  ingredients-service.go ingredient list + grocery tags
db/                      connection, migrations (embedded into the binary)
models/                  request/response structs shared by handlers and lib
internal/testutil/       test database setup for the Go tests
index.html               the whole page
static/js/
  main.js                page setup, recipe list, recipe popups, editing
  filter.js              search / filter / tag filters for the recipe list
  grocery-list.js        building the grocery list
  conversions.js         cups <-> grams/ml (density table)
  temperature.js         °F <-> °C in recipe steps
tests/                   frontend tests (node --test tests/)
scripts/                 test-backend.sh, backup-db.sh
gouda-woulda/            old self-hosted ollama model server (separate Go module)
```

# Backend tests
scripts/test-backend.sh

scripts/test-backend.sh -v -run TestUpdateRecipe

- needs the db container running (`docker compose up -d db`)
- runs against a separate `guac_test` database (created automatically), never the real `guac` one
- DeepInfra is replaced by a fake server, so tests never call the real API
- plain `go test ./...` still runs the non-database tests and skips the rest

# Frontend tests
node --test tests/

- plain node, no npm install needed (tests for conversions.js, temperature.js and grocery-list.js)

# Production
upgraded-guacamole.com

- running locally on raspberry pi with cloudflare tunnel

## project todo 
- update recipe edit flow
    - editing other users recipes -> for followers etc
    - add + remove ingredients improved flow

- continue testing deepinfra
    - maybe we can use a smarter/ more expensive text model for better parsing ?

- custom logging function
    - maintains two logs
        - all logs
        - error logs
    - still outputs docker logs

- maybe limit random users from uploading too many recipes
    - unknown user can upload as many recipes
    - only 5~ can be parsed until user is manually verified


    
## future implementations
# website stuff
- allow user to change users
- add cookie setup/warning to make complient (?)
- allow user to select that they are cooking a certain recipe (shows other people? or alerts them?)
- remove followers somehow
    -> maybe have an expandable list that shows the current followers
    -> maybe edit the add follower button to expand a list for showing current followers + adding new followers
- allow copying recipes over to your own so you can edit them
    - setup edit/deleting recipes only if you own them
    -> this should override the other recipe for user so it will only show one
    -> allow "deleting recipes for followed users" so you can handpick certain recipes

# recipe stuff
- send website link and parse that way
- auto suggest ingredients when typing to add them into list (trie datastructure)
    -> could use for manually adding ingredients to recipe as well
- add a season json so produce can be given season automattically
- show seasonal recipes (list ingredients by season, get current in season stuff?)
- allow adding images to recipes

# grocery list
- allow adding items multiple times
