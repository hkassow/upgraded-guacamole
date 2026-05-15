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

1. finish setting up oauth
    a. no user ==> create
2. backup db
3. allow following other users or viewing their recipes ?
4. get recipes for user only 



## future implementations
- for grocery list group same named ingredient with amount next to each other 
- allow adding items multiple times
- add a season json so produce can be given season automattically
- send website link and parse that way
- auto suggest ingredients when typing to add them into list (trie datastructure)
- show seasonal recipes (list ingredients by season, get current in season stuff?)
- photo upload
- allow listing ingredients without tags
- allow user to select that they are cooking a certain recipe (shows other people? or alerts them?)

