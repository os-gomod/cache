package redis

// casLuaScript atomically replaces a value when it matches the expected one.
const casLuaScript = `
local current = redis.call("GET", KEYS[1])
if current == false then return 0 end
if current ~= ARGV[1] then return 0 end
if ARGV[3] == "0" then
    redis.call("SET", KEYS[1], ARGV[2])
else
    redis.call("SET", KEYS[1], ARGV[2], "PX", ARGV[3])
end
return 1
`

// getSetPersistLuaScript atomically sets a key with no TTL and returns the old value.
const getSetPersistLuaScript = `
local old = redis.call("GET", KEYS[1])
redis.call("SET", KEYS[1], ARGV[1])
return old
`

// unlockIfValueMatchesLuaScript releases a lock only when the stored token matches.
const unlockIfValueMatchesLuaScript = `
local current = redis.call("GET", KEYS[1])
if current == false then return 0 end
if current ~= ARGV[1] then return 0 end
redis.call("DEL", KEYS[1])
return 1
`
