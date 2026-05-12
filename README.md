## upgraded-guacamole



# Run locally
go run .

# Run using docker
docker compose up --build

docker compose down

# Access db
docker exec -it guac psql -U postgres -d guac

## project todo 

1. add table for user (just id + name (?)) + update table to include user option for recipe
2. backup db
3. for grocery list group same named ingredient with amount next to each other 
4. allow adding items multiple times
5. add a season json so produce can be given season automattically


## future implementations
- send website link and parse that way
- auto suggest ingredients when typing to add them into list (trie datastructure)
- show seasonal recipes (list ingredients by season, get current in season stuff?)
- photo upload
- allow listing ingredients without tags


