#!/usr/bin/env python3
"""Deterministic scientific plots from checked source values and designer intent."""
import json,sys,textwrap
from pathlib import Path
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
import numpy as np
from matplotlib.patches import Rectangle

root=Path(sys.argv[1]).resolve();src=json.loads((root/'inputs/source.json').read_text());story=json.loads((root/'inputs/story.json').read_text())['content'];spec=json.loads((root/'output/spec.json').read_text())['content'];theme=story['design'];out=root/'output/graphics';out.mkdir(parents=True,exist_ok=True)
plt.rcParams.update({'font.family':'DejaVu Sans','font.size':13,'axes.spines.top':False,'axes.spines.right':False,'axes.spines.left':False,'axes.edgecolor':'#B8C4CE','text.color':theme['ink'],'axes.labelcolor':theme['ink'],'xtick.color':theme['ink'],'ytick.color':theme['ink'],'svg.fonttype':'none'})
matrix=src['matrix'];rows=matrix['totals'];names=[r['name'] for r in rows];colors=[theme['accent'],theme['secondary'],'#8B6BA5','#A5894F','#477B9E','#75804B']*2
def wrap(x,n=65):return '\n'.join(textwrap.wrap(x,n))
for v in spec['visuals']:
 fig=plt.figure(figsize=(12,7.5),facecolor=theme['paper']);ax=fig.add_axes([.19,.24,.74,.49]);ax.set_facecolor(theme['paper']);kind=v['kind']
 fig.text(.07,.91,wrap(v['title'],62),fontsize=24,weight='bold',va='top');fig.text(.07,.795,wrap(v['subtitle'],105),fontsize=12,color='#526476',va='top')
 note=''
 if kind=='score_bounds':
  for i,r in enumerate(rows):
   ax.barh(i,r['lower_bound'],color=colors[i],height=.48)
   gap=r['upper_bound']-r['lower_bound']
   if gap:ax.barh(i,gap,left=r['lower_bound'],color='#DBE2E7',height=.48,hatch='///',edgecolor='#8C9BA6',linewidth=0)
   ax.text(min(r['upper_bound']+1.6,97),i,f"{r['lower_bound']:g}–{r['upper_bound']:g}",va='center',fontsize=12,weight='bold')
  ax.set_yticks(range(len(rows)),names);ax.invert_yaxis();ax.set_xlim(0,108);ax.set_xticks([0,25,50,75,100]);ax.set_xlabel('Weighted fit / 100');ax.grid(axis='x',alpha=.18);ax.set_axisbelow(True)
  note='Solid: supported points. Hatched: unresolved score potential. These bounds describe missing evidence.'
 elif kind=='coverage':
  vals=[r['coverage_percent'] for r in rows];ax.barh(names,vals,color=colors[:len(rows)],height=.5);ax.invert_yaxis();ax.set_xlim(0,110);ax.set_xticks([0,25,50,75,100]);ax.set_xlabel('Criteria weight supported by evidence (%)')
  for i,val in enumerate(vals):ax.text(val+1,i,f'{val:g}%',va='center',weight='bold')
  note='Coverage measures scored criterion weight. It does not measure source truth or confidence in a vendor.'
 elif kind=='score_heatmap':
  criteria=matrix['criteria'];lookup={(c['candidate_id'],c['criterion_id']):c for c in matrix['cells']};data=np.array([[np.nan if lookup[r['candidate_id'],c['id']]['score'] is None else lookup[r['candidate_id'],c['id']]['score'] for c in criteria] for r in rows])
  ax.set_position([.19,.28,.70,.44]);cmap=plt.get_cmap('Blues').copy();cmap.set_bad('#EAECEF');ax.imshow(data,vmin=0,vmax=5,cmap=cmap,aspect='auto');ax.set_yticks(range(len(rows)),names);ax.set_xticks(range(len(criteria)),[wrap(c['name'],16)+'\n'+str(c['weight'])+'%' for c in criteria],fontsize=10)
  for i in range(len(rows)):
   for j in range(len(criteria)):
    val=data[i,j];ax.text(j,i,'?' if np.isnan(val) else f'{val:g}',ha='center',va='center',weight='bold',color='white' if val>=3 else theme['ink'],fontsize=17)
  note='Scores use criterion-specific anchors, 0–5. A question mark indicates missing evidence, never a zero.'
 elif kind=='effects':
  tests=[t for t in src['statistics']['comparisons'] if t['status']=='inferential'];ax.remove()
  for i,t in enumerate(tests):
   if len(tests)>3:raise ValueError('Effects graphic supports at most three comparisons; select another encoding or extend the reviewed renderer')
   a=fig.add_axes([.18,.64-i*(.45/len(tests)),.72,.45/len(tests)-.08]);a.set_facecolor(theme['paper']);d=t['mean_difference_a_minus_b'];a.errorbar(d,0,xerr=[[d-t['ci_low']],[t['ci_high']-d]],fmt='o',color=colors[i],capsize=8,linewidth=3,markersize=9);a.axvline(0,color='#8C9BA6',linestyle='--');a.set_yticks([]);a.set_title(f"{t['metric_id']} · {t['groups'][0]} minus {t['groups'][1]}",loc='left',fontsize=12);a.set_xlabel(t['unit'],loc='right');a.margins(x=.2)
  note='Marginal confidence intervals at the level declared in the statistical plan; separate unit scales. Not simultaneous intervals.'
 elif kind=='sensitivity':
  scenarios=matrix['sensitivity'];ax.set_position([.23,.24,.69,.49])
  for i,r in enumerate(rows):ax.plot([s['lower_bounds'][r['candidate_id']] for s in scenarios],np.arange(len(scenarios)),'.-',label=r['name'],color=colors[i],linewidth=1.5,markersize=7)
  ax.set_yticks(range(len(scenarios)),[s['criterion_id']+' × '+str(s['factor']) for s in scenarios],fontsize=9);ax.invert_yaxis();ax.set_xlabel('Supported weighted score / 100');ax.legend(frameon=False,loc='lower right');ax.grid(axis='x',alpha=.2)
  note='Each scenario changes one weight by ±20% and renormalizes. This is preference sensitivity, not sampling uncertainty.'
 elif kind=='decision_path':
  ax.remove();steps=v['steps'];n=len(steps)
  for i,step in enumerate(steps):
   x=.075+i*(.90/n);fig.text(x,.66,f'{i+1:02d}',fontsize=30,color=theme['accent'],weight='bold');fig.text(x,.55,wrap(step['heading'],22 if n==4 else 28),fontsize=15,weight='bold',va='top');fig.text(x,.36,wrap(step['detail'],29 if n==4 else 37),fontsize=11,color='#526476',va='top')
  note='Read from the supported finding through its conditions. This is an explanatory sequence, not an approved delivery plan.'
 fig.text(.07,.12,wrap(v['caption'],120),fontsize=10.5,va='top');fig.text(.07,.035,wrap(note,135),fontsize=9,color='#526476',va='bottom')
 fig.savefig(out/(v['id']+'.svg'),facecolor=fig.get_facecolor());fig.savefig(out/(v['id']+'.png'),dpi=150,facecolor=fig.get_facecolor());plt.close(fig)
