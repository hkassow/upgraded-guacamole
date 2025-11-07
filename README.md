## upgraded-guacamole



# Run locally
go run .

# Run using docker
docker build -t my-go-api .

docker run -p 8443:8443 my-go-api


## project goals 


# view/create/edit recipes
# create grocery list from recipes 
# organize ingredients by tags (tags self created like upstairs and downstairs for our use case)

## technical details

# need database (mysql running in container?)
# need tables (recipe, ingredient w/ tag metadata?)
# need frontend (vite?) 
# go backend


## future implementations
# grocery list export to google keeps?
# host website on web behind some type of auth?
# show seasonal recipes (list ingredients by season, get current in season stuff?)
# photo upload
# export recipes from sites? (paste link and then copy recipe?)
# allow listing ingredients without tags
# have ingredient lister that shows which need tags



# steps
[x] 1. enable hot reload
2. setup views for main recipe page
3. setup views for creating recipe page

