# Diff of two schema dumps (db_all.tsv, ent_all.tsv in the given dir);
# see scripts/schema_drift.sh.
import sys, collections
d=sys.argv[1]
def load(p):
    rows=[l.rstrip('\n').split('\t') for l in open(p)]
    # schema_migrations is the migration runner's own table, not ent's.
    rows=[r for r in rows if len(r)>1 and r[1]!='schema_migrations']
    C={}; I=collections.defaultdict(set); F=collections.defaultdict(set)
    for r in rows:
        if r[0]=='C': C[(r[1],r[2])]=(r[3].replace('tinyint(1)','tinyint'),r[4],r[5].replace('now()','CURRENT_TIMESTAMP'),r[6].replace(' on update CURRENT_TIMESTAMP','~onupdate'))
        elif r[0]=='I': I[r[1]].add((r[2],r[3],r[5]))
        elif r[0]=='F': F[r[1]].add((r[2],r[3],r[4],r[5]))
    return C,I,F
pc,pi,pf=load(d+'/db_all.tsv'); ec,ei,ef=load(d+'/ent_all.tsv')
ptables={k[0] for k in pc}; etables={k[0] for k in ec}
print('tables only in the database:', sorted(ptables-etables)); print('tables only in ent:', sorted(etables-ptables))
cnt=collections.Counter()
for k in sorted(set(pc)|set(ec)):
    if k[0] not in ptables&etables: continue
    a=pc.get(k); b=ec.get(k)
    if a!=b:
        cnt[k[0]]+=1
        print('COL', k[0], k[1], 'db=',a,'ent=',b)
for t in sorted(ptables&etables):
    for x in sorted(pi[t]-ei[t]): print('IDX db-only', t, x)
    for x in sorted(ei[t]-pi[t]): print('IDX ent-only ', t, x)
    for x in sorted(pf[t]-ef[t]): print('FK  db-only', t, x)
    for x in sorted(ef[t]-pf[t]): print('FK  ent-only ', t, x)
