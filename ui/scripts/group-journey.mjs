// Actual embedded desktop + authenticated phone routes; isolated fake-provider fixture.
// After make embed, start from repo root:
// CHUCK_GROUP_BROWSER_READY=/tmp/groups.json go test -tags sqlite_fts5 ./internal/server -run '^TestGroupBrowserFixture$' -timeout 20m -v
// Then: node ui/scripts/group-journey.mjs /tmp/groups.json /tmp/groups-run
// Finish the fixture by creating /tmp/groups.json.done after the browser exits.
import { readFileSync, mkdirSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { chromium } from "playwright";
const ready = process.argv[2];
const fixture = JSON.parse(readFileSync(ready, "utf8"));
const out = process.argv[3] || "/tmp/chuck-group-journey";
mkdirSync(out, { recursive: true });
const browser = await chromium.launch();
const receipts = [];
const errors = [];
const check = (name, pass) => { receipts.push({name, pass}); if (!pass) throw new Error(name); console.log(`PASS ${name}`); };
async function api(path, body, method = body === undefined ? "GET" : "POST") {
  const r = await fetch(fixture.desktop + path, {method, headers:{"Content-Type":"application/json"}, body: body === undefined ? undefined : JSON.stringify(body)});
  const data = await r.json();
  if (!r.ok) throw new Error(`${path}: ${JSON.stringify(data)}`);
  return data;
}
const launch = async (name, group, project = "my-app") => (await api("/api/sessions", {name,group,project,role:"implementer",backend:"claude",model:"sonnet",interface:"chat"})).agent.agent_id;
const desktopContext = await browser.newContext({viewport:{width:1024,height:900}});
const phoneContext = await browser.newContext({viewport:{width:390,height:844},ignoreHTTPSErrors:true});
await phoneContext.addCookies([{name:fixture.cookie,value:fixture.token,url:fixture.phone,secure:true,httpOnly:true,sameSite:"Strict"}]);
const desktop = await desktopContext.newPage();
const phone = await phoneContext.newPage();
for (const page of [desktop,phone]) page.on("pageerror", e => errors.push(e.message));
try {
  // The loopback static fallback rejects unknown POSTs with 405; neither status admits a route.
  check("legacy group route absent on desktop", [404,405].includes((await desktopContext.request.post(fixture.desktop + "/api/groups/auth/release", { data: {} })).status()));
  check("legacy group route absent on phone", (await phoneContext.request.post(fixture.phone + "/api/groups/auth/release", { headers: { Origin: fixture.phone }, data: {} })).status() === 404);
  const projects = await api("/api/projects");
  await api("/api/projects/my-app", {...projects["my-app"],cwd:"/tmp"}, "PUT");
  for (const session of await api("/api/sessions")) await api(`/api/sessions/${session.agent_id}/archive`,{},"POST");
  for (const skin of ["", "sky-grove", "studio"]) {
    const name = skin || "core";
    await api("/api/config", {appearance_skin:skin}, "PUT");
    const builder = await launch(`Builder ${name}`, "Build/review");
    const reviewer = await launch(`Reviewer ${name}`, "Destination with a very long group name for readable controls");
    const other = await launch(`Other ${name}`, "Build/review", "other");
    const ungrouped = await launch(`Ungrouped ${name}`, "");
    await api(`/api/sessions/${reviewer}/stop`,{},"POST");
    await desktop.goto(fixture.desktop + "/project/my-app");
    await phone.goto(fixture.phone + "/project/my-app");
    const source = desktop.getByRole("region",{name:"Group Build/review",exact:true});
    const destination = desktop.getByRole("region",{name:"Group Destination with a very long group name for readable controls",exact:true});
    await source.getByText(`Builder ${name}`,{exact:true}).waitFor();
    await phone.getByRole("region",{name:"Build/review group",exact:true}).waitFor();
    await phone.getByRole("region",{name:"Ungrouped group",exact:true}).getByText(`Ungrouped ${name}`,{exact:true}).waitFor();
    await desktop.screenshot({path:join(out,`${name}-desktop-1024.png`),fullPage:true});
    await phone.screenshot({path:join(out,`${name}-phone-390.png`),fullPage:true});
    await desktop.getByRole("button",{name:"New agent",exact:true}).click();
    await desktop.getByText("Options",{exact:true}).click();
    const newGroup = desktop.getByRole("combobox",{name:"Group",exact:true});
    await newGroup.fill(`New group ${name}`);
    await desktop.getByRole("button",{name:`Create group “New group ${name}”`,exact:true}).click();
    await desktop.getByRole("textbox",{name:"Name",exact:true}).focus();
    await newGroup.focus();
    await desktop.screenshot({path:join(out,`${name}-desktop-picker.png`),fullPage:true});
    await desktop.getByRole("dialog").getByRole("button",{name:"Cancel",exact:true}).click();
    await desktop.getByRole("dialog").waitFor({state:"detached"});
    if(await destination.getAttribute("data-state")!=="collapsed") await destination.locator("header > button").first().click();
    const grip = source.getByRole("button",{name:`Reorder Builder ${name}`});
    const from = await grip.boundingBox();
    const to = await destination.locator("header").boundingBox();
    await desktop.mouse.move(from.x+from.width/2,from.y+from.height/2);
    await desktop.mouse.down();
    await desktop.mouse.move(to.x+to.width/2,to.y+to.height/2,{steps:20});
    await desktop.mouse.up();
    await desktop.screenshot({path:join(out,`${name}-after-drop.png`),fullPage:true});
    await phone.getByRole("region",{name:"Destination with a very long group name for readable controls group",exact:true}).getByText(`Builder ${name}`,{exact:true}).waitFor();
    check(`${name}: collapsed-header cross-status drop updates phone`, (await api(`/api/sessions/${builder}`)).agent.group === "Destination with a very long group name for readable controls");
    await phone.getByText(`Builder ${name}`,{exact:true}).click();
    await phone.getByRole("tab",{name:"Manage",exact:true}).click();
    const picker = phone.getByRole("combobox",{name:"Group",exact:true});
    await picker.fill("Build/review");
    await phone.getByRole("button",{name:"Build/review",exact:true}).click();
    await phone.getByRole("button",{name:"Save group",exact:true}).click();
    await source.getByText(`Builder ${name}`,{exact:true}).waitFor();
    check(`${name}: phone reassignment updates desktop`,true);
    await phone.goto(fixture.phone + "/project/my-app");
    const phoneSource = phone.getByRole("region",{name:"Build/review group",exact:true});
    await phoneSource.getByRole("button",{name:"Stop group",exact:true}).click();
    check(`${name}: confirmation focuses Cancel`, await phone.getByRole("button",{name:"Cancel",exact:true}).evaluate(el=>el===document.activeElement));
    await phone.getByRole("dialog").getByRole("button",{name:"Stop group",exact:true}).click();
    await phone.getByRole("status",{name:"Group action result"}).waitFor();
    await desktop.getByText(`Builder ${name}`,{exact:true}).locator("xpath=ancestor::article").getByText("Stopped",{exact:true}).waitFor();
    check(`${name}: Stop group isolates other project`, (await api(`/api/sessions/${other}`)).running !== null);
    await source.getByRole("button",{name:"Archive group",exact:true}).click();
    await desktop.getByRole("dialog").getByRole("button",{name:"Archive group",exact:true}).click();
    await desktop.getByRole("status",{name:"Group action result"}).waitFor();
    await phoneSource.waitFor({state:"detached"});
    check(`${name}: archive keeps project and peer group`, !(await api("/api/projects"))["my-app"].archived && !(await api(`/api/sessions/${other}`)).agent.archived);
    await api(`/api/sessions/${builder}/restore`,{},"POST");
    await source.getByText(`Builder ${name}`,{exact:true}).waitFor();
    check(`${name}: individual restore reaches live desktop`,true);
    for (const width of [360,390,430]) {
      await phone.setViewportSize({width,height:844});
      check(`${name}: phone ${width}px no overflow`, await phone.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
      await phone.screenshot({path:join(out,`${name}-phone-${width}-result.png`),fullPage:true});
    }
    await desktop.setViewportSize({width:1440,height:900});
    await desktop.screenshot({path:join(out,`${name}-desktop-1440.png`),fullPage:true});
    check(`${name}: desktop no overflow`,await desktop.evaluate(()=>document.documentElement.scrollWidth<=innerWidth));
    await desktop.setViewportSize({width:1024,height:900});
    for (const id of [builder,reviewer,other,ungrouped]) await api(`/api/sessions/${id}/archive`,{},"POST");
  }
  check("no browser exceptions",errors.length===0);
} finally {
  writeFileSync(join(out,"report.json"),JSON.stringify({receipts,errors},null,2));
  await browser.close();
}
