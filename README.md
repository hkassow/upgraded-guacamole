## upgraded-guacamole
A recipe parser for storing recipes, uses vanilla go backend and vanilla javascript backend. 
Currently uses deepinfra api with Qwen models to handle image and text parsing

# Run locally using docker
docker compose up --build

docker compose down

# Production
upgraded-guacamole.com

# Access db
docker exec -it guac psql -U postgres -d guac

## project todo 
-. fix editing other peoples recipes
-. allow removing/adding ingredients from recipe
-. add alternative measurements in grocery list (cups -> grams etc)
    -> maintain list
        1. things that dont need to be seperated out (eggs)
        2. things that should be in ML ie; liquids, milk, cream etc
        3. things that should be in grams ie; butter, flour, sugar
-. surface alt ingredient amounts 1/2 cup (500g) etc
-. continue testing deepinfra
-. fix model interperting ingredients needed from recipe list
    -> see matcha & red bean recipe


-. filter recipe types
    - your recipes
    - recipes of people you follow
    - show all recipes
    - allow tagging recipes for further filtering
        - vegan
        - dinner
        - lunch
        - dessert
        - baking 

-. custom logging function
    - maintains two logs
        -> all logs
        -> error logs
    - still outputs docker logs

-. add a error state to to-be parsed recipes
    -> this way recipe wont continue to be retried
    -> add field for storing output of recipes

-. maybe limit random users from uploading too many recipes
    -> unknown user can upload as many recipes
    -> only 5~ can be parsed until user is manually verified


    
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