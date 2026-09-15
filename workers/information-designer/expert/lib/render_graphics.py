#!/usr/bin/env python3
"""Deterministic scientific plots from checked source values and designer intent."""
import json,sys,textwrap,math
from pathlib import Path
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.patches import Rectangle
from publication_contract import display_number
import numpy as np
from matplotlib.patches import Rectangle
from matplotlib.font_manager import FontProperties

root=Path(sys.argv[1]).resolve();docmode='--document' in sys.argv;src=json.loads((root/'inputs/source.json').read_text());story=json.loads((root/'inputs/story.json').read_text())['content'];spec=json.loads((root/('inputs/design.json' if docmode else 'output/spec.json')).read_text())['content'];theme=story['design'];out=root/('build/document-graphics' if docmode else 'output/graphics');out.mkdir(parents=True,exist_ok=True)
plt.rcParams.update({'font.family':theme['font'],'font.size':13,'axes.spines.top':False,'axes.spines.right':False,'axes.spines.left':False,'axes.edgecolor':'#B8C4CE','text.color':theme['ink'],'axes.labelcolor':theme['ink'],'xtick.color':theme['ink'],'ytick.color':theme['ink'],'svg.fonttype':'none'})
matrix=src['matrix'];rows=matrix['totals'];names=[r['name'] for r in rows];colors=[theme['accent'],theme['secondary'],'#8B6BA5','#A5894F','#477B9E','#75804B']*2
def wrap(x,n=65):return '\n'.join(textwrap.wrap(x,n))
def fitted(text,fig,size,width,weight='normal'):
 fig.canvas.draw();renderer=fig.canvas.get_renderer();font=FontProperties(family=theme['font'],size=size,weight=weight);lines=[];line=''
 for word in text.split():
  candidate=(line+' '+word).strip()
  if line and renderer.get_text_width_height_descent(candidate,font,False)[0]>fig.bbox.width*width:lines.append(line);line=word
  else:line=candidate
 if line:lines.append(line)
 return '\n'.join(lines)
notes={}
for v in spec['visuals']:
 kind=v['kind'];height=3.7 if kind=='effects' else 3.6 if kind=='decision_path' else 2.8 if kind in ['score_bounds','coverage','score_heatmap'] else 3.9
 fig=plt.figure(figsize=(6.8,height) if docmode else (12,7.5),facecolor=theme['paper']);ax=fig.add_axes([.23,.28,.68,.50] if docmode else [.19,.24,.74,.49]);ax.set_facecolor(theme['paper'])
 if not docmode:fig.text(.07,.94,fitted(v['title'],fig,24,.86,'bold'),fontsize=24,weight='bold',va='top');fig.text(.07,.79,fitted(v['subtitle'],fig,12,.86),fontsize=12,color='#526476',va='top')
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
   pitch=.42/len(tests);a=fig.add_axes([.18,.74-pitch-i*pitch,.72,pitch-.10]);a.set_facecolor(theme['paper']);d=t['mean_difference_a_minus_b'];a.errorbar(d,0,xerr=[[d-t['ci_low']],[t['ci_high']-d]],fmt='o',color=colors[i],capsize=8,linewidth=3,markersize=9);a.axvline(0,color='#8C9BA6',linestyle='--');a.set_yticks([]);count=f"{t['paired_count']} matched pairs" if t['paired_count'] is not None else f"n={t['n_a']} / {t['n_b']}";label=f"{t['metric_id']}: {t['groups'][0]} minus {t['groups'][1]} ({t['unit']})\nMean {display_number(d)}; {100*t['confidence_level']:g}% CI [{display_number(t['ci_low'])}, {display_number(t['ci_high'])}]; {count}; Holm p={t['p_holm']:.4g}";a.set_title(label,loc='left',fontsize=10.5)
   for ref in v.get('reference_lines',[]):
    if ref['comparison_id']==t['id']:
     a.axvline(ref['value'],color=theme['secondary'],linestyle=':',linewidth=2);a.annotate(f"Practical {ref['value']:g} {t['unit']}",(ref['value'],.94),xycoords=('data','axes fraction'),xytext=(4,-2),textcoords='offset points',fontsize=8 if docmode else 9,va='top',ha='left',bbox={'facecolor':'white','edgecolor':'none','alpha':.85,'pad':1})
   a.margins(x=.2)
  levels=', '.join(f"{x:g}%" for x in sorted({t['confidence_level']*100 for t in tests}));note=f'Marginal {levels} confidence intervals; separate unit scales. Not simultaneous or multiplicity-adjusted intervals.'
 elif kind=='sensitivity':
  scenarios=matrix['sensitivity'];ax.set_position([.23,.24,.69,.49])
  markers=['o','s','^','D','v','P','X','*','h','<','>','p']
  for i,r in enumerate(rows):ax.plot([s['lower_bounds'][r['candidate_id']] for s in scenarios],np.arange(len(scenarios)),linestyle='none',marker=markers[i],label=r['name'],color=colors[i],markersize=7 if docmode else 9)
  cn={c['id']:c['name'] for c in matrix['criteria']};ax.set_yticks(range(len(scenarios)),[wrap(cn[s['criterion_id']],23)+' × '+str(s['factor']) for s in scenarios],fontsize=9);ax.invert_yaxis();ax.set_xlabel('Supported weighted score / 100');ax.grid(axis='x',alpha=.2)
  if not docmode:ax.set_position([.23,.24,.69,.47-.033*math.ceil(len(rows)/3)]);fig.legend(*ax.get_legend_handles_labels(),frameon=False,loc='upper center',bbox_to_anchor=(.59,.745),ncol=min(3,len(rows)),fontsize=10)
  note='Each scenario changes one weight by ±20% and renormalizes. This is preference sensitivity, not sampling uncertainty.'
 elif kind=='decision_path':
  ax.remove();steps=v['steps'];n=len(steps)
  for i,step in enumerate(steps):
   if docmode:
    x=.035+(i%2)*.50;y=.96-(i//2)*.49;fig.text(x,y,fitted(step['heading'],fig,11,.43,'bold'),fontsize=11,weight='bold',va='top');fig.text(x,y-.13,fitted(step['detail'],fig,10,.43),fontsize=10,color='#526476',va='top')
   else:
    x=.075+i*(.90/n);fig.add_artist(Rectangle((x-.009,.28),.90/n-.012,.39,transform=fig.transFigure,facecolor='#EDF1F4',edgecolor='none',zorder=-1));fig.text(x,.625,fitted(step['heading'],fig,15,.90/n-.03,'bold'),fontsize=15,weight='bold',color=theme['accent'],va='top');fig.text(x,.48,fitted(step['detail'],fig,11,.90/n-.05),fontsize=11,color=theme['ink'],va='top')
  note='Each panel explains a decision condition or responsibility. Read the labeled conditions together.'
 if docmode:
  for a in fig.axes:
   a.tick_params(labelsize=9);a.xaxis.label.set_size(9);a.yaxis.label.set_size(9)
   for label in a.texts:label.set_fontsize(10)
  if kind=='score_heatmap':ax.set_position([.23,.30,.70,.64])
  elif kind in ['score_bounds','coverage','sensitivity']:ax.set_position([.27,.20,.63,.70])
  elif kind=='effects':
   pitch=.80/len(fig.axes)
   for i,a in enumerate(fig.axes):a.set_position([.15,.96-pitch-i*pitch,.80,pitch-.16]);a.title.set_fontsize(9)
  if kind=='sensitivity':ax.set_position([.38,.16,.52,.73-.045*math.ceil(len(rows)/3)]);fig.legend(*ax.get_legend_handles_labels(),frameon=False,loc='upper center',bbox_to_anchor=(.64,.98),ncol=min(3,len(rows)),fontsize=9)
 else:fig.text(.07,.155,fitted(v['caption'],fig,10.5,.86),fontsize=10.5,va='top');fig.text(.07,.045,fitted(note,fig,9,.86),fontsize=9,color='#526476',va='top')
 notes[v['id']]=note
 fig.savefig(out/(v['id']+'.svg'),facecolor=fig.get_facecolor());fig.savefig(out/(v['id']+'.png'),dpi=300 if docmode else 150,facecolor=fig.get_facecolor());plt.close(fig)
if docmode:(out/'notes.json').write_text(json.dumps(notes))
