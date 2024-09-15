# postshortly

## Purpose
postshortly is an api to post status updates using ed25519 keys without a signup process. The philosophy behind this is to provide an open-login system where users can authenticate themselves using ed25519 keys.

## Usage
### Posting a Status Update
To post a status update, send a POST request to the `/status` endpoint with a JSON payload containing the status update details. The payload must include the body of the status, an optional link, the public key, and the signature.

### Running Your Own Instance
To run your own instance of postshortly, clone the repository and run the `main.go` file. The service will start on port 3495 by default.

## API Routes & Methods
- `POST /status`: Create a new status update.
- `GET /status/{pubkey}`: Retrieve all status updates for a specific public key.
- `GET /status`: Retrieve all status updates.
- `GET /stats`: Retrieve statistics about the status updates and requests.

## Curl Examples
- To post a status update:
  ```sh
  curl -X POST http://localhost:3495/status -d '{"body":"Hello, world!","link":"","pubkey":"<public_key>","signature":"<signature>"}'
  ```

- To get status updates by public key:
  ```sh
  curl http://localhost:3495/status/<public_key>
  ```

- To get all status updates:
  ```sh
  curl http://localhost:3495/status
  ```

- To get statistics:
  ```sh
  curl http://localhost:3495/stats
  ```

## Signing and Verification
When posting a status update, the payload must be signed using the ed25519 private key corresponding to the provided public key. The data that is signed includes the concatenation of the public key, the body of the status, and the optional link. This ensures the integrity and authenticity of the status update.

## License
MIT License 2024 donuts-are-good, for more info see license.md