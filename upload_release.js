const https = require('https');
const fs = require('fs');

const token = process.env.GITHUB_TOKEN;
const repo = "codeforcrack-kalai/sudopulse-cloud-connector";
const tag = "v1.0.0";
const file = "build/sudopulse-connector-linux";
const assetName = "sudopulse-connector-linux";

const headers = {
  'Authorization': `Bearer ${token}`,
  'Accept': 'application/vnd.github+json',
  'User-Agent': 'NodeJS',
  'X-GitHub-Api-Version': '2022-11-28'
};

function request(method, url, data = null, isBinary = false) {
  return new Promise((resolve, reject) => {
    const reqHeaders = { ...headers };
    if (isBinary) {
      reqHeaders['Content-Type'] = 'application/octet-stream';
      reqHeaders['Content-Length'] = fs.statSync(data).size;
    }
    const req = https.request(url, { method, headers: reqHeaders }, res => {
      let body = '';
      res.on('data', d => body += d);
      res.on('end', () => {
        if (!body) return resolve({});
        try { resolve(JSON.parse(body)); } catch(e) { resolve(body); }
      });
    });
    req.on('error', reject);
    if (data && !isBinary) req.write(JSON.stringify(data));
    if (data && isBinary) {
        const fileStream = fs.createReadStream(data);
        fileStream.pipe(req);
    } else {
        req.end();
    }
  });
}

async function run() {
  console.log(`Creating/fetching release ${tag}...`);
  let release = await request('GET', `https://api.github.com/repos/${repo}/releases/tags/${tag}`);

  if (release.id) {
    const existingAsset = release.assets.find(a => a.name === assetName);
    if (existingAsset) {
      console.log(`Deleting existing asset ${existingAsset.id}...`);
      await request('DELETE', `https://api.github.com/repos/${repo}/releases/assets/${existingAsset.id}`);
    }
  } else {
     release = await request('POST', `https://api.github.com/repos/${repo}/releases`, {
        tag_name: tag, name: tag, draft: false, prerelease: false
     });
  }

  console.log(`Uploading asset to Release ID ${release.id}...`);
  const uploadUrl = `https://uploads.github.com/repos/${repo}/releases/${release.id}/assets?name=${assetName}`;
  const result = await request('POST', uploadUrl, file, true);
  console.log('Upload complete!', result.name || result);
}
run();
