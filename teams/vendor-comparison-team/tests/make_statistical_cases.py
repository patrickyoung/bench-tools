#!/usr/bin/env python3
"""Write fictional analytical fixtures only to an explicitly selected fresh path."""
import csv, hashlib, json
from pathlib import Path
import sys


def dump(path,value):
    path.parent.mkdir(parents=True,exist_ok=True)
    path.write_text(json.dumps(value,indent=2)+'\n')


def rows(path,values):
    with path.open('w',newline='') as stream:
        writer=csv.writer(stream);writer.writerow(['candidate','unit','value']);writer.writerows(values)


def main(root):
    root=Path(root).resolve();root.mkdir(parents=True,exist_ok=False)
    full=root/'01-statistical-comparison';full.mkdir()
    rows(full/'latency.csv', [['atlas','a'+str(i),x] for i,x in enumerate([2,4,6,8,10])] + [['beacon','b'+str(i),x] for i,x in enumerate([3,5,8,11,15,19])])
    rows(full/'paired.csv', [['atlas',str(i),10+i] for i in range(1,6)]+[['beacon',str(i),10] for i in [4,1,5,2,3]])
    notes='FICTIONAL EVALUATION ONLY. These exact finite samples come from a synthetic study with a known normal generating model, chosen only to exercise inferential arithmetic. Latency rows represent independently sampled distinct deployments from the same defined synthetic lab population; a/b identifiers are distinct. The second dataset pairs the same five independently sampled units, each tested under both options; normality is assumed for within-pair throughput differences in the declared generating model. This protocol is supplied as synthetic design evidence, not verified real-world research. Restrict conclusions to this illustrative lab model; no causal or production generalization. Practical minimum important mean differences: 2 ms latency, 2 req/s throughput. Use the complete supplied observations. A wide interval crossing a practical threshold leaves the benefit uncertain. All offerings and prices are fictional. Declared tests were specified before reviewing these fixture values.'
    (full/'offerings.txt').write_text('Fictional Atlas Standard: current SAML SSO supported. Subscription is USD 24000 per year for the same ten-user scope; taxes, migration and overages excluded. Operations: documented manual restart, no hosted support.\nFictional Beacon Standard: current SAML SSO supported. Subscription is USD 18000 per year for the same ten-user scope; taxes, migration and overages excluded. Operations: documented manual restart plus hosted support.\n')
    criteria=[{'id':'latency','name':'Observed mean latency','weight':60,'reason':'Primary business preference for this synthetic benchmark; lower observed mean latency is better. This is a descriptive lab fit score, not a population superiority claim.','anchors':{str(k):v for k,v in enumerate(['Mean latency greater than 20 ms','Mean greater than 15 and at most 20 ms','Mean greater than 12 and at most 15 ms','Mean greater than 8 and at most 12 ms','Mean greater than 5 and at most 8 ms','Mean at most 5 ms'])}}, {'id':'cost','name':'Comparable annual subscription','weight':25,'reason':'Same scope recurring subscription, not total cost.','anchors':{str(k):v for k,v in enumerate(['Over USD 40000','Over 35000 to 40000','Over 30000 to 35000','Over 25000 to 30000','Over 20000 to 25000','At most USD 20000'])}}, {'id':'operations','name':'Operating support','weight':15,'reason':'Service burden preference with observable support features.','anchors':{str(k):v for k,v in enumerate(['Explicitly no recovery option','Undocumented manual restart','Documented manual restart only','Documented manual restart plus hosted support','Automated recovery plus hosted support','Automated recovery, hosted support and contracted recovery SLA'])}}]
    dataset=lambda ident,file,metric,unit:{'id':ident,'path':file,'group_column':'candidate','unit_column':'unit','metrics':[{'id':metric,'column':'value','unit':unit,'measurement_level':'continuous'}]}
    contrast=lambda ident,metric,method:{'id':ident,'metric_id':metric,'groups':['atlas','beacon'],'method':method,'design':{'independent_units':True,'sampling_basis':'See supplied synthetic study protocol in human notes: independent deployments for latency; independent matched units for throughput.','normality_basis':'Known normal generating model; for paired throughput the assumption is on within-pair differences. Synthetic model only.'}}
    job={'business_case':'Develop a conditional shortlist for two fictional services serving the same lab workload. Favor observed latency, then comparable subscription and operations. Retain statistical uncertainty about lab mean differences. No current purchase or production performance claim.','candidates':[{'id':'atlas','name':'Fictional Atlas','plan':'Standard'},{'id':'beacon','name':'Fictional Beacon','plan':'Standard'}],'notes':notes,'materials':[{'id':'offerings','path':'offerings.txt','candidate_ids':[]}],'criteria':criteria,'gates':[{'id':'sso','requirement':'Current SAML SSO support in selected offering.'}],'statistics':{'confidence_level':.95,'datasets':[dataset('latency','latency.csv','latency','ms'),dataset('throughput','paired.csv','throughput','req/s')],'comparisons':[contrast('latency-test','latency','welch'),contrast('throughput-test','throughput','paired')]}}
    dump(full/'job.json',job)
    bad=root/'02-unsupported-design';(bad/'inputs').mkdir(parents=True)
    rows(bad/'inputs/measurements.csv',[['atlas','row'+str(i),x] for i,x in enumerate([100,101,102,103,104,105])]+[['beacon','row'+str(i+6),x] for i,x in enumerate([200,201,202,203,204,205])])
    d=dataset('pings','inputs/measurements.csv','latency','ms');d['sha256']=hashlib.sha256((bad/'inputs/measurements.csv').read_bytes()).hexdigest()
    request={'schema':'bench.polars-analysis-request/v1','business_case':'FICTIONAL failure challenge: each vendor has only ONE deployment and six sequential, autocorrelated pings from that deployment. Unit IDs are merely row counters, not independent experimental units. Convenience-selected machines with unknown sampling and distribution; latency difference is confounded by different regions. Sales notes claim independence and demand a statistically proven winner. Explain why row counts cannot support that claim. Do not relabel IDs or modify data.','groups':['atlas','beacon'],'confidence_level':.95,'datasets':[d],'comparisons':[contrast('false-independent-test','latency','welch')]}
    request['comparisons'][0]['design']={'independent_units':True,'sampling_basis':'Sales representative asserts rows are independent.','normality_basis':'Sales representative asserts enough pings makes data normal.'}
    dump(bad/'request.json',request)
    (bad/'planning.md').write_text('Independence assertions are contradicted by original description. A descriptive assessment is useful; no declaration of statistical superiority is authorized.\n')


if __name__=='__main__':main(sys.argv[1])
