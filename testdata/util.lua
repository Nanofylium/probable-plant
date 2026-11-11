local M = {nome = "util em Lua"}
function M.dobro(x) return x * 2 end
function M.soma(...) local s = 0 for _, v in ipairs({...}) do s = s + v end return s end
return M
