include "complement";
# Independent pixel oracle, not a second slab/interval implementation.
def contains($x; $y):
  .x <= $x and $x < .x+.width and .y <= $y and $y < .y+.height;
def verify($w; $h):
  . as $edit | complement($w;$h) as $protected |
  all($protected[];
    .x>=0 and .y>=0 and .width>0 and .height>0 and
    .x+.width<=$w and .y+.height<=$h and
    all([.x,.y,.width,.height][]; floor==.)) and
  all(range(0;$w); . as $x |
    all(range(0;$h); . as $y |
      ([$protected[] | select(contains($x;$y))] | length) ==
      (if any($edit[]; contains($x;$y)) then 0 else 1 end)));
# Exhaustive singles and ordered pairs on 3x3; empty, full and overlapping unions.
[range(0;3) as $x | range(0;3) as $y |
 range(1;4-$x) as $w | range(1;4-$y) as $h |
 {x:$x,y:$y,width:$w,height:$h}] as $rects |
if ([] | verify(3;3)) and
   all($rects[]; [.] | verify(3;3)) and
   all($rects[]; . as $a | all($rects[]; [$a,.] | verify(3;3))) and
   ([{x:0,y:0,width:2,height:3},{x:1,y:2,width:4,height:2},
     {x:4,y:0,width:2,height:6}] | verify(6;6))
then {single_rectangles:($rects|length),ordered_pairs:(($rects|length)*($rects|length)),
      additional_cases:2,pixel_partition:"exact"}
else error("complement pixel partition failed") end
