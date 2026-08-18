## upgraded-guacamole



# Run locally
go run .

# Run using docker
docker compose up --build

docker compose down

# Production
upgraded-guacamole.com

# Access db
docker exec -it guac psql -U postgres -d guac

## project todo 

1. finish setting up user
    b. logout ==> clear cache
    c. remove followers somehow
    d. dont display everything in menu unless user is logged in
2. backup db
3. allow default recipe add 
    a. allow default functionality to create a recipe directly
    b. prompt user when they are using the ai auto fill create  
4. change recipe modal to be the whole screen for cooking
5. log when user is cooking a recipe (?)

## future implementations
- copy buttons for share-code + friend code
- filter recipe types
- add cookie setup/warning to make complient (?)
- for grocery list group same named ingredient with amount next to each other 
- allow adding items multiple times
- add a season json so produce can be given season automattically
- send website link and parse that way
- auto suggest ingredients when typing to add them into list (trie datastructure)
- show seasonal recipes (list ingredients by season, get current in season stuff?)
- photo upload
- allow listing ingredients without tags
- allow user to select that they are cooking a certain recipe (shows other people? or alerts them?)

