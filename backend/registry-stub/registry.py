from flask import Flask, jsonify, send_file, abort
from io import BytesIO

app = Flask(__name__)

ASSIGNMENTS = {
    # All hosts removed - no assignments
}

BUNDLES = { "seg-v1": b"DEMO_BUNDLE_BYTES" }

@app.get("/artifacts/for-host/<host_id>")
def for_host(host_id):
    return jsonify(ASSIGNMENTS.get(host_id, []))

@app.get("/bundles/<artifact_id>")
def bundle(artifact_id):
    data = BUNDLES.get(artifact_id)
    if not data: abort(404)
    return send_file(BytesIO(data), as_attachment=False,
                     download_name=f"{artifact_id}.bin",
                     mimetype="application/octet-stream")

@app.get("/healthz")
def health():
    return "ok", 200

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8090)
