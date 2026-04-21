n = int(input())
a = list(map(int, input().split()))
target = int(input())
seen = {}
for i, x in enumerate(a):
    if target - x in seen:
        print(seen[target - x], i)
        break
    seen[x] = i
