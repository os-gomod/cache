package redis

import "github.com/redis/go-redis/v9"

// Lua scripts for atomic Redis operations. These scripts are loaded once and
// executed via Eval, ensuring atomicity on the Redis server side.

// casScript implements CompareAndSwap:
//
//	KEYS[1] = key
//	ARGV[1] = old value
//	ARGV[2] = new value
//	ARGV[3] = TTL in seconds (0 = no expiry)
//
// Returns:
//
//	1  = swap performed successfully
//	-1 = key not found
//	0  = value mismatch
var casScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
if current == false then
    return -1
end
if current ~= ARGV[1] then
    return 0
end
if tonumber(ARGV[3]) > 0 then
    redis.call('SETEX', KEYS[1], ARGV[3], ARGV[2])
else
    redis.call('SET', KEYS[1], ARGV[2])
end
return 1
`)

// getSetScript implements GetSet with TTL support:
//
//	KEYS[1] = key
//	ARGV[1] = new value
//	ARGV[2] = TTL in seconds (0 = no expiry)
//
// Returns the old value (nil if key didn't exist).
var getSetScript = redis.NewScript(`
local old = redis.call('GET', KEYS[1])
if tonumber(ARGV[2]) > 0 then
    redis.call('SETEX', KEYS[1], ARGV[2], ARGV[1])
else
    redis.call('SET', KEYS[1], ARGV[1])
end
return old
`)
