# Streaming events preserve duplicate keys. Each new traversal edge must be new.
# End-container events move the cursor up one level; equal leaf keys also repeat.
reduce inputs as $e ({seen:{}, previous:[], roots:0, ok:true};
  if ($e|length) == 1 then .previous = $e[0][0:-2]
  else
    $e[0] as $p |
    .previous as $old |
    ([range(0; ([($p|length),($old|length)]|min)) |
      select($p[0:.+1] == $old[0:.+1])] | length) as $common |
    (if $p == $old then ([$common-1,0]|max) else $common end) as $start |
    if $p == [] then .roots += 1 else
      reduce range($start; $p|length) as $i (.;
        ($p[0:$i+1]|tojson) as $key |
        if .seen[$key] then .ok = false else .seen[$key] = true end)
    end |
    .previous = $p
  end) | .ok and .roots <= 1
