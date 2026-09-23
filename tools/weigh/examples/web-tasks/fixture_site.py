"""Synthetic browser tasks; no copied third-party pages or user data."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from contextlib import contextmanager
import threading


PAGES = {
    "/": """<title>Northstar workspace</title><h1>Northstar</h1>
        <nav><a href='/support'>Assistance</a> <a href='/plans'>Plans</a></nav>
        <p>Workspace tools for small teams.</p><a href='https://outside.invalid'>Leave site</a>""",
    "/help/index.html": """<title>Assistance</title><h1>Help center</h1>
        <a href='guides.html'>Product handbooks</a><a href='/'>Home</a>""",
    "/help/guides.html": """<title>Handbooks</title><h1>Product handbooks</h1>
        <a href='backup.html'>Keep an independent copy of your work</a>
        <a href='branding.html'>Make the workspace yours</a>""",
    "/help/backup.html": """<title>Workspace backup</title><h1>Export a backup</h1>
        <p>Open Workspace settings, select Data, then Export archive.</p>
        <p>Backup download links remain available for 14 days.</p>""",
    "/plans": """<title>Plans</title><p>Compare monthly and annual billing.</p><a href='/'>Home</a>""",
    "/stays": """<title>Work retreat stays</title><h1>Work retreat stays</h1>
        <p>Compare observed listing facts. Availability is illustrative.</p>
        <article hidden><h2>Hidden stale listing</h2></article>
        <main id='listings'></main>
        <script>
        document.querySelector('#listings').innerHTML = `
        <article><h2>Garden Studio</h2><p>Dedicated desk in a quiet courtyard. Cancel for a full refund until two days before arrival.</p><a href='/stays/garden'>Details</a></article>
        <article><h2>Festival Loft</h2><p>A lively shared lounge above a music club. Prepaid stays are nonrefundable.</p><a href='/stays/festival'>Details</a></article>
        <article><h2>Harbor Cabin</h2><p>A quiet private study overlooking the water. Ask the host about cancellation terms.</p><a href='/stays/harbor'>Details</a></article>`;
        </script>""",
    "/none": """<title>No links</title><p>Nothing about backups here.</p>""",
}


@contextmanager
def site():
    visits = []

    class Handler(BaseHTTPRequestHandler):
        def log_message(self, *_):
            pass

        def do_GET(self):
            visits.append(self.path)
            if self.path == "/support":
                self.send_response(302)
                self.send_header("Location", "/help/index.html")
                self.end_headers()
                return
            content = PAGES.get(self.path)
            body = (content or "<h1>Missing</h1>").encode()
            self.send_response(200 if content else 404)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield f"http://127.0.0.1:{server.server_port}", visits
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


if __name__ == "__main__":
    with site() as (address, _):
        print(address, flush=True)
        threading.Event().wait()
