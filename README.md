## upgraded-guacamole
A recipe parser for storing recipes, uses vanilla go backend and vanilla javascript backend. 
Currently uses deepinfra api with Qwen models to handle image and text parsing

# Run locally using docker
docker compose up --build

docker compose down

`127.0.0.1:8443`

# Backend tests
scripts/test-backend.sh

scripts/test-backend.sh -v -run TestUpdateRecipe

- needs the db container running (`docker compose up -d db`)
- runs against a separate `guac_test` database (created automatically), never the real `guac` one
- DeepInfra is replaced by a fake server, so tests never call the real API
- plain `go test ./...` still runs the non-database tests and skips the rest

# Frontend tests
node --test tests/

- plain node, no npm install needed (tests for static/js/conversions.js)

# Production
upgraded-guacamole.com

- running locally on raspberry pi with cloudflare tunnel

## project todo 
- update recipe edit flow
    - editing other users recipes -> for followers etc
    - add + remove ingredients improved flow

- add alternative measurements in grocery list (cups -> grams etc)
    - maintain list of density for conversion
        1. things that dont need to be seperated out (eggs)
        2. things that should be in ML ie; liquids, milk, cream etc
        3. things that should be in grams ie; butter, flour, sugar



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
