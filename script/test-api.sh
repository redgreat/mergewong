#!/bin/bash

# API 测试脚本
# 测试 MergeWong 的基本功能

BASE_URL="http://localhost:8080"
TOKEN=""

echo "======================================"
echo "MergeWong API 测试脚本"
echo "======================================"
echo ""

# 颜色定义
GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# 测试健康检查
echo "1. 测试健康检查..."
response=$(curl -s "${BASE_URL}/health")
if [[ $response == *"ok"* ]]; then
    echo -e "${GREEN}✓ 健康检查通过${NC}"
else
    echo -e "${RED}✗ 健康检查失败${NC}"
    exit 1
fi
echo ""

# 测试登录
echo "2. 测试用户登录..."
login_response=$(curl -s -X POST "${BASE_URL}/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "admin123"
  }')

if [[ $login_response == *"token"* ]]; then
    TOKEN=$(echo $login_response | grep -o '"token":"[^"]*"' | sed 's/"token":"\(.*\)"/\1/')
    echo -e "${GREEN}✓ 登录成功${NC}"
    echo "Token: ${TOKEN:0:50}..."
else
    echo -e "${RED}✗ 登录失败${NC}"
    echo "响应: $login_response"
    exit 1
fi
echo ""

# 测试获取用户信息
echo "3. 测试获取用户信息..."
profile_response=$(curl -s -X GET "${BASE_URL}/api/profile" \
  -H "Authorization: Bearer ${TOKEN}")

if [[ $profile_response == *"username"* ]]; then
    echo -e "${GREEN}✓ 获取用户信息成功${NC}"
    echo "用户信息: $profile_response" | jq '.' 2>/dev/null || echo "$profile_response"
else
    echo -e "${RED}✗ 获取用户信息失败${NC}"
    echo "响应: $profile_response"
fi
echo ""

# 测试数据库列表
echo "4. 测试获取数据库表列表..."
tables_response=$(curl -s -X GET "${BASE_URL}/api/db/system/tables" \
  -H "Authorization: Bearer ${TOKEN}")

if [[ $tables_response == *"users"* ]]; then
    echo -e "${GREEN}✓ 获取表列表成功${NC}"
    echo "表列表: $tables_response" | jq '.' 2>/dev/null || echo "$tables_response"
else
    echo -e "${RED}✗ 获取表列表失败${NC}"
    echo "响应: $tables_response"
fi
echo ""

# 测试查询数据
echo "5. 测试查询用户数据..."
query_response=$(curl -s -X POST "${BASE_URL}/api/db/system/query" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{
    "sql": "SELECT * FROM users",
    "params": [],
    "page": 1,
    "page_size": 10
  }')

if [[ $query_response == *"data"* ]]; then
    echo -e "${GREEN}✓ 查询数据成功${NC}"
    echo "查询结果: $query_response" | jq '.' 2>/dev/null || echo "$query_response"
else
    echo -e "${RED}✗ 查询数据失败${NC}"
    echo "响应: $query_response"
fi
echo ""

# 测试同步任务列表
echo "6. 测试获取同步任务列表..."
sync_response=$(curl -s -X GET "${BASE_URL}/api/sync/tasks?page=1&page_size=10" \
  -H "Authorization: Bearer ${TOKEN}")

if [[ $sync_response == *"data"* ]]; then
    echo -e "${GREEN}✓ 获取同步任务列表成功${NC}"
    echo "任务列表: $sync_response" | jq '.' 2>/dev/null || echo "$sync_response"
else
    echo -e "${RED}✗ 获取同步任务列表失败${NC}"
    echo "响应: $sync_response"
fi
echo ""

echo "======================================"
echo "测试完成！"
echo "======================================"
