"""Local validation for this example; no model or tool-library imports."""
import hashlib
import json
import os
import pathlib
import stat

MAX_IMAGE = 16 * 1024 * 1024
MAX_JSON = 1024 * 1024


def digest(data):
    return hashlib.sha256(data).hexdigest()


def read_regular(path, limit):
    path = pathlib.Path(path)
    fd = os.open(str(path), os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    with os.fdopen(fd, "rb") as stream:
        before = os.fstat(stream.fileno())
        if not stat.S_ISREG(before.st_mode) or before.st_size > limit:
            raise ValueError("not a bounded regular file: " + str(path))
        data = stream.read(limit + 1)
        after = os.fstat(stream.fileno())
    if len(data) > limit or (before.st_ino, before.st_size, before.st_mtime_ns) != (
            after.st_ino, after.st_size, after.st_mtime_ns):
        raise ValueError("file changed while reading: " + str(path))
    return data


def load_json(data):
    def pairs(items):
        obj = {}
        for key, value in items:
            if key in obj:
                raise ValueError("duplicate JSON key: " + key)
            obj[key] = value
        return obj
    def invalid(value):
        raise ValueError("non-JSON number: " + value)
    return json.loads(data.decode("utf-8"), object_pairs_hook=pairs, parse_constant=invalid)


def text(value):
    return isinstance(value, str) and bool(value.strip())


def sha256(value):
    return isinstance(value, str) and len(value) == 64 and all(c in '0123456789abcdef' for c in value)


def absolute_path(value):
    return text(value) and pathlib.Path(value).is_absolute()


def rubric_ids(rubric):
    if (not isinstance(rubric, dict) or set(rubric) != {"version", "criteria"}
            or type(rubric.get("version")) is not int or rubric["version"] != 1):
        raise ValueError("rubric requires version 1")
    rows = rubric.get("criteria")
    if not isinstance(rows, list) or not 1 <= len(rows) <= 128:
        raise ValueError("rubric requires criteria")
    ids = []
    for row in rows:
        if (not isinstance(row, dict) or set(row) != {"id", "requirement", "feedback"}
                or not all(text(row.get(k)) for k in ("id", "requirement", "feedback"))):
            raise ValueError("each criterion requires id, requirement, and feedback")
        if row["id"] in ids:
            raise ValueError("duplicate criterion id")
        ids.append(row["id"])
    return ids


def validate_observations(raw, ids, image_ids):
    if not isinstance(raw, dict) or set(raw) != {"observations", "limitations"}:
        raise ValueError("invalid observation fields")
    if (not isinstance(raw["limitations"], list) or not raw["limitations"]
            or not all(text(x) for x in raw["limitations"])):
        raise ValueError("limitations must contain at least one nonempty string")
    if not isinstance(raw["observations"], list):
        raise ValueError("observations must be an array")
    expected = {(criterion, image) for criterion in ids for image in image_ids}
    seen = set()
    for row in raw["observations"]:
        if not isinstance(row, dict) or set(row) != {"criterion_id", "image_id", "status", "observation"}:
            raise ValueError("invalid observation row")
        if not text(row["criterion_id"]) or not text(row["image_id"]):
            raise ValueError("observation ids must be strings")
        key = (row["criterion_id"], row["image_id"])
        if key not in expected or key in seen:
            raise ValueError("unknown or duplicate criterion/image observation")
        seen.add(key)
        if row["status"] not in ("observed", "uncertain", "not_visible") or not text(row["observation"]):
            raise ValueError("invalid observation status or text")
    if seen != expected:
        raise ValueError("missing criterion/image observation")


def validate_current(doc, rubric_data):
    if (not isinstance(doc, dict)
            or set(doc) != {"version", "kind", "rubric_sha256", "images", "observer", "observations", "limitations"}
            or type(doc.get("version")) is not int or doc["version"] != 1
            or doc.get("kind") != "visual-observations"):
        raise ValueError("invalid visual observation envelope")
    if not sha256(doc.get("rubric_sha256")) or digest(rubric_data) != doc["rubric_sha256"]:
        raise ValueError("rubric changed since observation")
    images = doc.get("images")
    if not isinstance(images, list) or not 1 <= len(images) <= 16:
        raise ValueError("invalid image bindings")
    ids = []
    for i, item in enumerate(images, 1):
        if (not isinstance(item, dict) or set(item) != {"id", "source", "snapshot", "sha256"}
                or item.get("id") != "image-%d" % i or not sha256(item.get("sha256"))):
            raise ValueError("invalid image id")
        ids.append(item["id"])
        for key in ("source", "snapshot"):
            if not absolute_path(item.get(key)):
                raise ValueError("image bindings must be absolute paths")
            if digest(read_regular(item[key], MAX_IMAGE)) != item.get("sha256"):
                raise ValueError("image changed: " + item[key])
    raw = {key: doc.get(key) for key in ("observations", "limitations")}
    validate_observations(raw, rubric_ids(load_json(rubric_data)), ids)
    observer = doc.get("observer")
    if (not isinstance(observer, dict)
            or set(observer) != {"tool", "model", "session", "record", "raw_output", "raw_sha256"}
            or observer.get("tool") != "ask" or not text(observer.get("model"))
            or not sha256(observer.get("raw_sha256"))):
        raise ValueError("missing observer metadata")
    if not all(absolute_path(observer[key]) for key in ("session", "record", "raw_output")):
        raise ValueError("observer evidence bindings must be absolute paths")
    output = read_regular(observer["raw_output"], MAX_JSON)
    if digest(output) != observer.get("raw_sha256") or load_json(output) != raw:
        raise ValueError("observer output changed")
    return raw
