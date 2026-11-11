local co = coroutine.create(function(a, b)
  print("início", a, b)
  local c = coroutine.yield(a + b)
  print("recebeu", c)
  local d, e = coroutine.yield(c * 2)
  print("recebeu", d, e)
  return "fim", 99
end)
print(coroutine.status(co))
print(coroutine.resume(co, 1, 2))
print(coroutine.status(co))
print(coroutine.resume(co, 10))
print(coroutine.resume(co, "x", "y"))
print(coroutine.status(co))
print((coroutine.resume(co)))
-- generator with wrap
local function gen(n)
  return coroutine.wrap(function()
    for i = 1, n do coroutine.yield(i) end
  end)
end
local s = 0
for v in gen(5) do s = s + v end
print("soma", s)
-- error inside coroutine
local bad = coroutine.create(function() error("falhou") end)
local ok, msg = coroutine.resume(bad)
print(ok, msg)
local bad2 = coroutine.create(function() error({code = 7}) end)
local ok2, e2 = coroutine.resume(bad2)
print(ok2, type(e2), e2.code)
-- running / isyieldable
print(coroutine.isyieldable())
local inner = coroutine.create(function()
  local c, main = coroutine.running()
  print(type(c), main, coroutine.isyieldable(), coroutine.status(c))
end)
coroutine.resume(inner)
-- producer / consumer
local function producer()
  return coroutine.create(function()
    for _, item in ipairs({"a", "b", "c"}) do coroutine.yield(item) end
  end)
end
local p = producer()
local items = {}
while true do
  local _, item = coroutine.resume(p)
  if item == nil then break end
  items[#items + 1] = item
end
print(table.concat(items, ","))
-- nested coroutines
local outer = coroutine.wrap(function()
  local innerc = coroutine.wrap(function() coroutine.yield(1); coroutine.yield(2) end)
  coroutine.yield(innerc() + innerc())
  coroutine.yield("depois")
end)
print(outer(), outer())
-- recursive generator (tree)
local function walk(t)
  if type(t) == "table" then for _, v in ipairs(t) do walk(v) end else coroutine.yield(t) end
end
local leaves = {}
for leaf in coroutine.wrap(function() walk({1, {2, {3, 4}}, 5}) end) do leaves[#leaves + 1] = leaf end
print(table.concat(leaves, " "))
-- close
local cl = coroutine.create(function() coroutine.yield() end)
coroutine.resume(cl)
print(coroutine.close(cl), coroutine.status(cl))
-- yield across pcall
local pc = coroutine.wrap(function()
  local ok, v = pcall(function() local x = coroutine.yield("dentro do pcall"); return x * 2 end)
  coroutine.yield(tostring(ok) .. " " .. v)
end)
print(pc())
print(pc(21))
