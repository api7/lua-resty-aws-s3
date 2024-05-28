local ffi = require("ffi")

local _M = {}

local function load_shared_lib(so_name)
  local string_gmatch = string.gmatch
  local string_match = string.match
  local io_open = io.open
  local io_close = io.close

  local cpath = package.cpath

  for k, _ in string_gmatch(cpath, "[^;]+") do
    local fpath = string_match(k, "(.*/)")
    fpath = fpath .. so_name
    local f = io_open(fpath)
    if f ~= nil then
      io_close(f)
      return ffi.load(fpath)
    end
  end
end

local go_s3 = load_shared_lib("s3.so") -- Load the shared library

ffi.cdef [[
  void *getObject(const char *cBucket, const char *cKey, const char *cRegion, const char *cAccessKey, const char *cSecretKey, const char *cCustomEndpoint);
]]

function _M.get_object(bucket, key, region, access_key, secret_key, custom_endpoint)
  local resp = go_s3.getObject(bucket, key, region, access_key, secret_key, custom_endpoint)
  return ffi.string(resp)
end
