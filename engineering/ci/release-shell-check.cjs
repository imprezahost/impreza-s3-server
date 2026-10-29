const fs=require('node:fs'),path=require('node:path'),cp=require('node:child_process'),yaml=require('js-yaml');
const root=process.cwd(),bash=process.argv[2];if(!bash)throw Error('existing bash path required');
let count=0;
for(const file of ['release.yml','reproducible-build.yml','vulncheck.yml']){
 const doc=yaml.load(fs.readFileSync(path.join(root,'.github/workflows',file),'utf8'));
 for(const [job,j] of Object.entries(doc.jobs))for(const s of j.steps||[])if(s.run){const r=cp.spawnSync(bash,['-n'],{input:s.run,encoding:'utf8',windowsHide:true});if(r.status!==0)throw Error(`${file}/${job}: ${r.stderr}`);count++;}
}
const signing=fs.readFileSync('engineering/release/SIGNING.md','utf8');for(const m of signing.matchAll(/```sh\r?\n([\s\S]*?)```/g)){const r=cp.spawnSync(bash,['-n'],{input:m[1],encoding:'utf8',windowsHide:true});if(r.status!==0)throw Error(r.stderr);count++;}
console.log(`PASS bash -n: ${count} workflow/signing script blocks; syntax only, no release or signature executed`);