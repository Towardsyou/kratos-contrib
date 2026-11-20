"use strict";

const byID = (id) => document.getElementById(id);
const file = byID("file");
const submit = byID("submit");
const uploadStatus = byID("upload-status");
const downloadStatus = byID("download-status");
const fileStatus = byID("file-status");
const files = byID("files");
let token = sessionStorage.getItem("upload-demo-token");

function status(element, message, error = false) {
  element.textContent = message;
  element.classList.toggle("error", error);
}

function updateIdentity() {
  byID("identity").textContent = token ? "已登录 · 测试用户" : "访客";
  byID("login").hidden = Boolean(token);
  byID("logout").hidden = !token;
}

function setToken(value) {
  token = value;
  if (value) {
    sessionStorage.setItem("upload-demo-token", value);
  } else {
    sessionStorage.removeItem("upload-demo-token");
  }
  updateIdentity();
}

async function api(path, body) {
  const headers = {};
  const requestToken = token;
  if (requestToken) headers.Authorization = "Bearer " + requestToken;
  if (body) headers["Content-Type"] = "application/json";
  const response = await fetch(path, {
    method: body ? "POST" : "GET",
    headers,
    body: body ? JSON.stringify(body) : undefined,
    cache: "no-store",
  });
  if (response.status === 401) {
    if (token === requestToken) {
      setToken(null);
      files.replaceChildren();
      status(fileStatus, "登录后查看文件列表。");
    }
    throw new Error("请先登录再继续。");
  }
  if (!response.ok) throw new Error("请求失败（HTTP " + response.status + "）。");
  return response.json();
}

async function objectRequest(request, body) {
  const headers = new Headers();
  for (const [name, header] of Object.entries(request.headers ?? {})) {
    for (const value of header.values) headers.append(name, value);
  }
  const response = await fetch(request.url, {
    method: request.method,
    headers,
    body,
    credentials: "omit",
    cache: "no-store",
  });
  if (!response.ok) throw new Error("存储请求失败（HTTP " + response.status + "）。");
  return response;
}

async function refreshFiles() {
  files.replaceChildren();
  if (!token) {
    status(fileStatus, "登录后查看文件列表。");
    return;
  }
  const requestToken = token;
  status(fileStatus, "正在加载文件列表…");
  try {
    const { uploads = [] } = await api("/v1/upload");
    if (token !== requestToken) return;
    status(fileStatus, uploads.length ? "" : "还没有上传的文件。");
    for (const upload of uploads) {
      const row = document.createElement("li");
      const name = document.createElement("span");
      name.textContent = upload.filename;
      const download = document.createElement("button");
      download.type = "button";
      download.textContent = "下载原文件";
      download.setAttribute("aria-label", "下载原文件 " + upload.filename);
      download.onclick = () => downloadFile(upload, download);
      row.append(name, download);
      files.append(row);
    }
  } catch (err) {
    if (token === requestToken) status(fileStatus, err.message, true);
  }
}

async function downloadFile(upload, button) {
  button.disabled = true;
  status(downloadStatus, "正在下载 " + upload.filename + "…");
  try {
    const result = await api("/v1/upload/" + encodeURIComponent(upload.key) + "/download");
    const response = await objectRequest(result.request);
    const url = URL.createObjectURL(await response.blob());
    const link = document.createElement("a");
    link.href = url;
    link.download = upload.filename;
    document.body.append(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    status(downloadStatus, "已下载 " + upload.filename);
  } catch (err) {
    status(downloadStatus, "下载失败。" + err.message, true);
  } finally {
    button.disabled = false;
  }
}

byID("login").onclick = async () => {
  const button = byID("login");
  button.disabled = true;
  try {
    setToken((await api("/demo/session", {})).token);
    status(uploadStatus, "");
    await refreshFiles();
  } catch (err) {
    status(uploadStatus, err.message, true);
  } finally {
    button.disabled = false;
  }
};
byID("logout").onclick = () => {
  setToken(null);
  status(uploadStatus, "");
  status(downloadStatus, "");
  void refreshFiles();
};
byID("refresh").onclick = () => location.reload();
byID("choose").onclick = () => file.click();
file.onchange = () => {
  byID("selected").textContent = file.files[0]?.name ?? "未选择文件";
  status(uploadStatus, "");
};

byID("upload").onsubmit = async (event) => {
  event.preventDefault();
  const selected = file.files[0];
  if (!selected) {
    status(uploadStatus, "请先选择文件。", true);
    return;
  }
  submit.disabled = true;
  status(uploadStatus, "正在上传 " + selected.name + "…");
  try {
    const { key, request } = await api("/v1/upload/presign", {
      filename: selected.name,
      contentType: selected.type || "application/octet-stream",
      size: selected.size,
    });
    await objectRequest(request, selected);
    const { upload } = await api("/v1/upload/complete", { key });
    if (upload.status !== "verified") throw new Error("文件尚未通过校验。");
    status(uploadStatus, "上传成功：" + selected.name);
    await refreshFiles();
  } catch (err) {
    status(uploadStatus, err.message, true);
  } finally {
    submit.disabled = false;
  }
};

updateIdentity();
void refreshFiles();
