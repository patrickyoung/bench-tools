include "complement";
# Inputs: exact job, selected style ID, independently queried bounds, and
# canonical hashes. XML ID uniqueness is separately checked by trusted shell.
def box: {x:.[0],y:.[1],width:.[2],height:.[3]};
def overlaps($a; $b):
  $a.x < $b.x+$b.width and $b.x < $a.x+$a.width and
  $a.y < $b.y+$b.height and $b.y < $a.y+$a.height;
def queried($id; $w; $h):
  [$bounds[0][] | select(.id==$id)] as $found |
  if ($found|length) != 1 then error("missing/duplicate queried target ID: "+$id)
  else $found[0].box end |
  if length != 4 or (all(.[]; type=="number" and isfinite)|not)
  then error("invalid queried target bounds: "+$id) else . end |
  if .[0]<0 or .[1]<0 or .[2]<=0 or .[3]<=0 or
     .[0]+.[2]>$w or .[1]+.[3]>$h
  then error("zero/outside queried target bounds: "+$id) else box end;
. as $job |
(.vector_request.width // 1200) as $w |
(.vector_request.height // 900) as $h |
(.styles[] | select(.id==$id)) as $style |
($style.target_padding // 0) as $pad |
[$style.target_ids[] | . as $tid |
  queried($tid; $w; $h) as $b |
  # Outward rounding protects whole raster pixels; padding clips to canvas.
  ([$b.x-$pad | floor, 0] | max) as $x |
  ([$b.y-$pad | floor, 0] | max) as $y |
  ([$b.x+$b.width+$pad | ceil, $w] | min) as $right |
  ([$b.y+$b.height+$pad | ceil, $h] | min) as $bottom |
  {id:$tid,measured_box:$b,
   edit_box:{x:$x,y:$y,width:($right-$x),height:($bottom-$y)}}] as $targets |
[$job.required_text[] | select(.id as $tid | $style.target_ids | index($tid))] as $selected |
if any($job.required_text[];
    . as $text |
    ($style.target_ids | index($text.id) | not) and
    (queried($text.id; $w; $h) as $b | any($targets[]; overlaps($b; .edit_box))))
then error("target intersects unselected required text")
else . end |
([$targets[].edit_box] | complement($w; $h)) as $preserve |
if ($preserve|length)==0 then error("target union covers entire frame")
elif ($preserve|length)>256 then error("target complement exceeds 256 preserve regions")
else
 {version:1,style_id:$id,target_ids:$style.target_ids,target_padding:$pad,
  canvas:{width:$w,height:$h},targets:$targets,preserve_regions:$preserve,
  text_ids_allowed_to_change:[$selected[].id],selected_wording:$selected,
  canonical_master_sha256:$master,canonical_svg_sha256:$svg,canonical_png_sha256:$png,
  protection_scope:"all pixels outside the union of expanded rectangular target boxes; selected lettering requires visual verification"}
end
