# Integer half-open boxes. Vertical slabs, merged covered y intervals, then
# horizontal coalescing of identical uncovered strips. No image decoding.
def complement($w; $h):
  . as $boxes |
  ([0,$w] + [$boxes[] | .x, (.x+.width)] | unique) as $xs |
  [range(0; ($xs|length)-1) as $i |
    $xs[$i] as $x | $xs[$i+1] as $end |
    ([$boxes[] | select(.x < $end and .x+.width > $x) |
       [.y, .y+.height]] | sort_by(.[0], .[1]) |
     reduce .[] as $b ({end:0,gaps:[]};
       if $b[0] > .end then .gaps += [[.end,$b[0]]] else . end |
       .end = ([.end,$b[1]] | max))) |
    (.gaps + (if .end < $h then [[.end,$h]] else [] end))[] |
    {x:$x,y:.[0],width:($end-$x),height:(.[1]-.[0])}] |
  group_by([.y,.height]) |
  [ .[] | sort_by(.x) |
    reduce .[] as $r ([];
      if length > 0 and (.[-1].x+.[-1].width == $r.x)
      then .[-1].width += $r.width else . + [$r] end) | .[] ] |
  sort_by(.x,.y,.width,.height) |
  map(. + {label:"Outside selected target boxes"});
