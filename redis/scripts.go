package redis

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

const getSetPersistLuaScript = `
local old = redis.call("GET", KEYS[1])
redis.call("SET", KEYS[1], ARGV[1])
return old
`

const unlockIfValueMatchesLuaScript = `
local current = redis.call("GET", KEYS[1])
if current == false then return 0 end
if current ~= ARGV[1] then return 0 end
redis.call("DEL", KEYS[1])
return 1
`
