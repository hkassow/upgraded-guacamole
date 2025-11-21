## upgraded-guacamole



# Run locally
go run .

# Run using docker
docker compose up --build

docker compose down

## project goals 

1. add recipes
   [x] a. parse recipes
   [x] b. allow async recipe upload (dont wait for recipe to be parsed just complete and come back later)
    
2. create grocery list from recipes
    a. add tags to ingredients
    b. combine similar tags together
    c. add lone ingredients

3. filter for recipes/ingredients 


## future implementations
- send website link and parse that way
- auto suggest ingredients when typing to add them into list (trie datastructure)
- grocery list export to google keeps?
- host website on web behind some type of auth?
- show seasonal recipes (list ingredients by season, get current in season stuff?)
- photo upload
- export recipes from sites? (paste link and then copy recipe?)
- allow listing ingredients without tags
- have ingredient lister that shows which need tags




# database additions
category table
id, name, description(?)
    
    (dinner, lunch, breakfast, snack, dessert, soup?)

recipe_category table
id, recipe_id, category_id

comment table (?)
id, recipe_id, text


