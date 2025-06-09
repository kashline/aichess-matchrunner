AI Chess Matchrunner

This worker handles generating the moves for all matches in the redis match queue.  Once a match is finished, it also updates the database with the results.

To run locally:
Create a '.env' file with the following variables:
DATABASE_URL
STOCKFISH_URL
OPENAI_API_KEY

Then run the build script
`./build.sh`
