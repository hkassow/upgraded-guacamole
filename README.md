## upgraded-guacamole
A simple recipe parser for storing recipes, uses vanilla go backend and vanilla javascript backend. With Ollama running on a seperate computer/server for parsing text/images and converting to recipes.
# Run locally using docker
docker compose up --build

docker compose down

# Production
upgraded-guacamole.com

# Access db
docker exec -it guac psql -U postgres -d guac

## project todo 
-. allow uploading multiple pictures for a single recipe
-. fix editing other peoples recipes
-. fix grocery list adding to just do dairy.meat.dry.produce
-. look into using runpod to run my qwen models
-. add backend tests
    - create recipe manual
    - create recipe ai fill (mock ai response)
    - update recipe
    - update ingredients
    - delete recipe 
    - add follower
    - remove follower (when added)

-. filter recipe types
    - your recipes
    - recipes of people you follow
    - show all recipes

-. custom logging function
    - maintains two logs
        -> all logs
        -> error logs
    - still outputs docker logs
    
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
- allow recipes to have multiple ingredient sections
    -> add db row to ingredient for grouping

# grocery list
- for grocery list group same named ingredient with amount next to each other 
- allow adding items multiple times

# unsure
- allow listing ingredients without tags