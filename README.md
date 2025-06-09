# aichess-matchrunner

This worker handles generating the moves for all matches in the redis match queue.  Once a match is finished, it also updates the database with the results.

## Local build:

### Env variables
Create a '.env' file with the following variables:
DATABASE_URL
STOCKFISH_URL
OPENAI_API_KEY

### Required infrastructure
You'll need these available and their respective urls placed in .env:
Postgres
Redis

Then run the build script
`./build.sh`
