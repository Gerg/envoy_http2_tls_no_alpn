#!/usr/bin/env python3

from quart import Quart, request

app = Quart(__name__)


@app.route("/")
async def hello():
    message = f"Hello, {request.full_path}; HTTP Version: {request.http_version}."
    print(message)
    return message


if __name__ == "__main__":
    app.run(port=8080)
