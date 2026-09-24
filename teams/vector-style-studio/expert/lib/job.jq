# Strict case data, not instructions. Coordinates are original PNG pixels.
def keys_are($required; $optional):
  type == "object" and ((keys - ($required + $optional)) | length == 0)
  and (($required - keys) | length == 0);
def integer: type == "number" and floor == .;
def text($n): type == "string" and utf8bytelength <= $n and test("\\S")
  and (explode | all(. != 0));
def sid: type == "string" and test("^[a-z][a-z0-9-]{0,31}\\z");
def tid: type == "string" and test("^[A-Za-z][A-Za-z0-9_-]{0,63}\\z");
def path: type == "string" and startswith("/") and length <= 1024
  and (explode | all(. >= 32 and . != 127 and . != 92))
  and (split("/")[1:] | all(. != "" and . != "." and . != ".."));
def dimension: integer and . >= 256 and . <= 4096;
def job:
  (.vector_request.width // 1200) as $w |
  (.vector_request.height // 900) as $h |
  keys_are(["version","vector_request","preserve_regions","required_text","styles"]; ["source_svg"])
  and .version == 1
  and (.vector_request |
    keys_are(["description"]; ["style","width","height"])
    and (.description | type == "string" and length >= 1 and length <= 12000 and (explode | all(. != 0)))
    and ((has("style") | not) or .style == null or
      (.style | type == "string" and length >= 1 and length <= 2000 and (explode | all(. != 0))))
    and ((has("width") | not) or (.width | dimension))
    and ((has("height") | not) or (.height | dimension)))
  and ((has("source_svg") | not) or (.source_svg | path))
  and $w * $h <= 8388608
  and (.preserve_regions | type == "array" and length <= 256 and all(
    keys_are(["x","y","width","height","label"]; [])
    and (.x | integer and . >= 0) and (.y | integer and . >= 0)
    and (.width | integer and . > 0) and (.height | integer and . > 0)
    and .x + .width <= $w and .y + .height <= $h
    and (.label | text(256))))
  and (.required_text | type == "array" and length <= 256 and all(
    keys_are(["id","text"]; []) and (.id | tid) and (.text | text(8000)))
    and (map(.id) | length == (unique | length)))
  and (.styles | type == "array" and length >= 1 and length <= 8 and all(
    keys_are(["id","style"]; []) and (.id | sid and . != "original") and (.style | text(8000)))
    and (map(.id) | length == (unique | length)));
# Exact union area by horizontal slabs and merged y intervals, no pixel decoder.
def union_area:
  . as $rs | ([0] + [$rs[] | .x, (.x + .width)] | unique) as $xs
  | [range(0; ($xs|length)-1) as $i |
    ([$rs[] | select(.x <= $xs[$i] and .x + .width >= $xs[$i+1])
      | [.y, .y + .height]] | sort_by(.[0])
      | reduce .[] as $a ({end:0, n:0};
          .n += ([$a[1] - ([.end,$a[0]] | max),0] | max)
          | .end = ([.end,$a[1]] | max)) | .n) * ($xs[$i+1]-$xs[$i])
    ] | add // 0;
select(job) |
select((.preserve_regions | union_area) <
  ((.vector_request.width // 1200) * (.vector_request.height // 900))) |
. as $job |
select(all(.styles[];
  ({version:1,source:"inputs/source.png",style:.style,preserve_regions:$job.preserve_regions}
   | tojson | utf8bytelength) < 65536))
