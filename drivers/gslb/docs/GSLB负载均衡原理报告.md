# GSLB 负载均衡原理报告

## 1. 概述

### 1.1 背景

GSLB（Global Server Load Balancing，全局服务器负载均衡）驱动原有的调度算法为"优先级淘汰制 + 故障转移"：按 ASN → ASO → ISP → CountryCode 逐级比较，匹配的节点排在前面，获取链接时依次尝试，第一个成功即返回。

这种机制存在一个核心缺陷：**当多个节点配置完全相同时（如同一运营商的多个存储节点），永远只有配置顺序最前的节点承担流量，其余节点仅在故障时作为 fallback。**

### 1.2 目标

在保持原有智能路由能力的基础上，实现同优先级节点间的负载均衡，使流量能够在等价节点间均匀分配。

### 1.3 设计原则

1. **向后兼容**：不改变现有配置的默认行为，新功能通过新增字段 opt-in
2. **最小改动**：在现有排序框架上扩展，而非重写调度引擎
3. **显式控制**：通过 `balance` 和 `balance_universal` 字段明确声明参与负载均衡的节点
4. **智能提升**：万能节点（全球 CDN）在缺少完美匹配时自动提升优先级，在已有完美匹配时保持低调

---

## 2. 原有调度机制分析

### 2.1 原有排序算法

```
优先级 1: ASN（自治系统号码）      精确匹配
优先级 2: Aso（自治系统名称）      模糊匹配（子串，不区分大小写）
优先级 3: ISP（运营商）            前缀匹配（不区分大小写）
优先级 4: CountryCode（国家代码）  精确匹配
```

排序采用淘汰制：逐轮比较，一旦分出胜负立即决定顺序，不再继续。

### 2.2 原有选择行为

```go
for i, s := range sorted {
    link, o, err := d.link(ctxChild, rp, args)
    if err != nil {
        continue  // 失败才试下一个
    }
    return &resultLink, nil  // 成功立即返回
}
```

**关键**：第一个成功即返回，不会尝试后续节点。

### 2.3 原有机制的局限

**场景**：两个电信节点配置完全相同

```yaml
- path: /ty/h
  asn: [4134]
  country_code: ["CN"]

- path: /ty/p
  asn: [4134]
  country_code: ["CN"]
```

电信用户访问时，两者排序优先级完全相同，保持原配置顺序 → `/ty/h` 永远排第一 → `/ty/h` 承担 100% 流量，`/ty/p` 承担 0%。

---

## 3. 新调度算法设计

### 3.1 计分体系

将原有的多维度淘汰制改为**二维计分制**：

| 维度 | 分值 | 说明 |
|------|------|------|
| Carrier Score | 0 或 1 | ASN/ASO/ISP 任意命中即为 1 |
| Country Score | 0、1 或 2 | 命中=2，无配置=1，不命中=0 |
| **Total Score** | **0 ~ 3** | Carrier + Country |

#### 3.1.1 Carrier Score 计算

ASN、ASO、ISP 三者本质上都是"运营商匹配"的不同表达维度：

- **ASN**：自治系统号码，来自 GeoLite2-ASN 数据库，精确匹配
- **ASO**：自治系统名称，来自 GeoLite2-ASN 数据库，模糊匹配（子串，不区分大小写）
- **ISP**：运营商名称，来自 qqwry.ipdb 数据库，前缀匹配（不区分大小写）

三者任意一个命中即说明该节点为该运营商服务，因此**不叠加计分**：

```
Carrier Score:
  if ASN 命中 or ASO 命中 or ISP 命中:
      return 1
  else:
      return 0
```

**为什么 ASN/ASO/ISP 不叠加？**

1. ASN 和 ASO 来自同一数据库，是同一信息的两种表达，命中 ASN 时 ASO 通常也命中，叠加会虚高分数
2. 三者描述的是同一个属性（运营商），多个弱匹配不应超过一个强匹配
3. 简单的 0/1 计分避免了权重调参的复杂性

#### 3.1.2 Country Score 计算

Country Score 采用三档计分，核心思想是区分"明确服务"、"通用"和"明确不服务"：

| 条件 | Country Score | 含义 |
|------|---------------|------|
| country_code 匹配 或 country_code_not 未排除 | **2** | 明确服务该地区用户 |
| 无任何 country 配置 | **1** | 通用节点，不限制地区 |
| country_code_not 排除 且 country_code 不匹配 | **0** | 明确不服务该地区用户 |

**匹配流程**（先 not 后 code）：

```
Country Score:
  if 节点配置了 country_code_not:
      if 用户国家 在 country_code_not 列表中:
          return 0  // 被排除
      else:
          return 2  // 用户不在排除列表中
  elif 节点配置了 country_code:
      if 用户国家 在 country_code 列表中:
          return 2  // code 命中
      else:
          return 0  // 不匹配
  else:
      return 1  // 无配置，通用节点
```

**为什么先匹配 country_code_not？**

`country_code` 和 `country_code_not` 是两种互斥的匹配策略——白名单（我**只服务**这些国家）和黑名单（我**不服务**这些国家）。当同时配置时，`country_code_not` 优先，`country_code` 被忽略。因为"非"的范围一定比"指定"的范围更大，先匹配 not 可以避免逻辑冲突。

**建议**：不要同时配置 `country_code` 和 `country_code_not`。所有同时配置的场景都可以简化为只用一个字段：

| 同时配置 | 等价简化 |
|---------|---------|
| `code: [HK, TW]` + `not: [CN]` | `code: [HK, TW]`（CN 本来就不在列表里） |
| `code: [HK, TW, MO, CN]` + `not: [CN]` | `code: [HK, TW, MO]`（把 CN 从列表里删掉） |
| `not: [CN]` + `code: [HK, TW]` | `not: [CN]`（not 已经覆盖了 HK/TW） |

**为什么 Country=2 而 Carrier=1？**

Country 的权重高于 Carrier，基于以下实际考量：

1. **国际互联优于国际到国内**：境外移动子网用户（如巴基斯坦 Zong，ASO 含 "China Mobile"）匹配了国内移动节点的运营商，但国家不匹配。实际网络中，从巴基斯坦到中国国内节点的链路质量远差于到境外节点。Country=2 > Carrier=1 确保境外用户优先走境外节点
2. **国家维度的信号更强**：Country 匹配意味着节点在该地区有部署，是比运营商匹配更基础的服务能力保证
3. **避免同分歧义**：如果 Carrier=1 + Country=0 和 Carrier=0 + Country=1 同分，无法区分"运营商匹配但国家不匹配"和"国家匹配但运营商不匹配"两种截然不同的场景

### 3.2 负载均衡机制

#### 3.2.1 balance 字段

```yaml
- path: /ty/h
  asn: [4134]
  country_code: ["CN"]
  balance: true       # 加入负载均衡组

- path: /ty/p
  asn: [4134]
  country_code: ["CN"]
  balance: true       # 同分，与 /ty/h 随机选择
```

`balance: true` 的节点在**同分**时参与组内随机选择，而非按配置顺序固定。

**规则**：
- 只有 `balance: true` 的节点参与随机
- 同分中 `balance: false`（默认）的节点保持原配置顺序，排在随机组之后
- 随机组内节点数 >= 2 才进行随机，否则保持原序

#### 3.2.2 balance_universal 字段

```yaml
- path: /全球CDN
  balance_universal: true    # 万能节点，条件性提升优先级
```

`balance_universal` 用于标记全球 CDN 等无地区/运营商限制的万能节点。其核心语义是：**"当没有完美匹配时，我和最佳选项一样好。"**

**行为规则**：

1. `balance_universal: true` 隐含 `balance: true`，无需同时配置
2. 条件性分数提升（boost）：

```
boost 目标 = 非 universal 节点中的最高分

对每个 balance_universal 节点：
  if Country Score > 0（未被显式排除）
     且 boost目标 <= 2（没有完美匹配）
     且 boost目标 > 自然分：
      实际分 = boost目标
  else：
      实际分 = 自然分
```

**boost 条件解析**：

| 条件 | 原因 |
|------|------|
| Country > 0 | Country=0 意味着被 country_code_not 排除，不应提升，否则绕过排除意图 |
| boost目标 <= 2 | 最高分 >= 3 说明已有 Carrier+Country 完美匹配，万能节点不应抢占 |
| boost目标 > 自然分 | 自然分已经不低于目标时无需提升 |

**为什么 boost 目标是"非 universal 节点的最高分"？**

如果包含 universal 节点自身，多个 universal 节点可能互相抬高，形成虚假高分。以非 universal 节点为锚点，确保 boost 后的分数反映的是"与当前最佳专用节点同等优先"。

### 3.3 完整算法流程

```
┌─────────────────────────────────────────────────────────┐
│                    1. 过滤阶段                          │
│  排除 NoDown、MinSize、MaxSize 不满足的节点             │
└────────────────────────┬────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────┐
│                    2. 计分阶段                          │
│  对每个节点：                                           │
│    a. 计算 Country Score（先 not 后 code）              │
│    b. 计算 Carrier Score（ASN/ASO/ISP 任意命中=1）     │
│    c. 自然分 = Carrier + Country                       │
└────────────────────────┬────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────┐
│                    3. Boost 阶段                        │
│  确定 boost 目标 = 非 universal 节点中的最高分          │
│  对每个 balance_universal 节点：                        │
│    if Country > 0 且 boost目标 <= 2 且 boost目标 > 自然分: │
│        实际分 = boost目标                               │
│    else:                                               │
│        实际分 = 自然分                                  │
└────────────────────────┬────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────┐
│                    4. 排序阶段                          │
│  4a. 按实际分降序排序                                   │
│  4b. 同分时按原有优先级 tie-breaker：                   │
│      ASN > ASO > ISP > CountryCode                     │
│      （分出子组，只有 tie-breaker 相同的节点才视为等价） │
│  4c. 等价子组内：                                       │
│      - balance/balance_universal 节点随机打乱(>=2个)    │
│      - 非 balance 节点保持原配置顺序                    │
│      - 排列：[随机 balance 组] + [原序 non-balance 组]  │
└────────────────────────┬────────────────────────────────┘
                         ▼
┌─────────────────────────────────────────────────────────┐
│                    5. 选择阶段                          │
│  按最终顺序依次尝试获取链接                             │
│  首个成功即返回，失败则尝试下一个                       │
└─────────────────────────────────────────────────────────┘
```

---

## 4. 配置参数详解

### 4.1 新增字段

#### `balance`（负载均衡）

| 属性 | 值 |
|------|-----|
| 类型 | 布尔值 |
| 默认 | `false` |
| 作用 | 标记该节点参与同分负载均衡 |

当多个 `balance: true` 的节点得分相同时，在这些节点间随机选择，而非按配置顺序固定选择第一个。

**使用场景**：同一运营商的多个存储节点需要分摊流量。

```yaml
- path: /ty/h
  asn: [4134]
  country_code: ["CN"]
  balance: true       # 与 /ty/p 分摊电信流量

- path: /ty/p
  asn: [4134]
  country_code: ["CN"]
  balance: true       # 与 /ty/h 分摊电信流量
```

#### `balance_universal`（万能负载均衡）

| 属性 | 值 |
|------|-----|
| 类型 | 布尔值 |
| 默认 | `false` |
| 作用 | 标记该节点为万能节点，条件性提升优先级并参与负载均衡 |
| 隐含 | `balance: true` |

万能节点（如全球 CDN）在缺少完美匹配时自动提升至与最佳专用节点同分，参与负载均衡；在已有完美匹配时保持自然分，不抢占专用节点的流量。

**使用场景**：全球 CDN 节点，无地区/运营商限制，作为通用加速源。

```yaml
- path: /全球CDN
  balance_universal: true    # 无配置限制，条件性提升
```

#### `country_code_not`（国家排除）

| 属性 | 值 |
|------|-----|
| 类型 | 字符串数组 |
| 默认 | 空 |
| 匹配方式 | 精确匹配 |
| 作用 | 排除特定国家/地区的用户 |

匹配优先级高于 `country_code`：先检查 `country_code_not`，用户在排除列表中则 Country=0；用户不在排除列表中则 Country=2，跳过 `country_code` 检查。

**校验规则**：`country_code_not` 与 `country_code` 互斥，同时配置时应在初始化阶段报错拒绝加载（Fail-fast），而非运行时静默忽略。

**使用场景**：境外专用节点，明确不服务国内用户。

```yaml
- path: /境外节点
  country_code_not: ["CN"]   # 服务所有非 CN 用户
```

### 4.2 字段交互关系

```
country_code_not ──用户在排除列表中──→ Country=0，跳过 country_code
                └─用户不在排除列表中──→ Country=2，跳过 country_code

country_code ─────匹配───→ Country=2
              └─不匹配──→ Country=0

无 country 配置 ──────────→ Country=1

balance_universal: true ──隐含──→ balance: true
```

---

## 5. 场景验证

### 5.1 标准负载均衡

**配置**：
```yaml
- path: /ty/h
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /ty/p
  asn: [4134]
  country_code: ["CN"]
  balance: true
```

**CN 电信用户（ASN=4134, Country=CN）**：

| 节点 | Carrier | Country | 总分 | balance | 结果 |
|------|---------|---------|------|---------|------|
| /ty/h | 1 | 2 | 3 | true | 同分随机组 |
| /ty/p | 1 | 2 | 3 | true | 同分随机组 |

**行为**：/ty/h 和 /ty/p 各 50% 概率被选中。选中后失败则 fallback 到另一个。

---

### 5.2 balance 与 non-balance 混合

**配置**：
```yaml
- path: /A
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /B
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /C
  asn: [4134]
  country_code: ["CN"]
  # 无 balance
```

**CN 电信用户**：

| 节点 | 总分 | balance | 结果 |
|------|------|---------|------|
| /A | 3 | true | 随机组 |
| /B | 3 | true | 随机组 |
| /C | 3 | false | 原序排列在随机组之后 |

**行为**：A 和 B 随机选择，C 只在 A、B 都失败时才被尝试。

---

### 5.3 不同分节点

**配置**：
```yaml
- path: /电信节点
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /国内通用
  country_code: ["CN"]
  balance: true

- path: /境外节点
  country_code_not: ["CN"]
  balance: true
```

**CN 电信用户**：

| 节点 | Carrier | Country | 总分 |
|------|---------|---------|------|
| /电信节点 | 1 | 2 | **3** |
| /国内通用 | 0 | 2 | **2** |
| /境外节点 | 0 | 0 | **0** |

**行为**：不同分，不在同一 balance 组。电信节点优先，失败 fallback 到国内通用，再失败 fallback 到境外节点。

---

### 5.4 境外移动子网用户

**背景**：巴基斯坦 Zong 是中国移动海外子公司，ASN 17808，ASO 包含 "China Mobile"，但 Country=PK。

**配置**：
```yaml
- path: /国内移动
  aso: ["China Mobile"]
  country_code: ["CN"]

- path: /境外节点
  country_code_not: ["CN"]

- path: /全球CDN
  balance_universal: true
```

**巴基斯坦 Zong 用户（ASO 含 "China Mobile", Country=PK）**：

| 节点 | Carrier | Country | 自然分 | boost | 实际分 |
|------|---------|---------|--------|-------|--------|
| /国内移动 | 1 | 0（PK≠CN） | 1 | - | **1** |
| /境外节点 | 0 | 2（PK不在not列表） | 2 | - | **2** |
| /全球CDN | 0 | 1 | 1 | 最高=2,boost | **2** |

**行为**：境外节点和全球CDN 同分=2，balance 组随机选择。国内移动 score=1 作为 fallback。

**分析**：国际互联比国际到国内更通畅，境外节点和全球CDN 优先于国内移动节点，符合实际网络状况。

---

### 5.5 国内电信用户 + 全球CDN

**配置**：
```yaml
- path: /电信节点
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /全球CDN
  balance_universal: true
```

**CN 电信用户（ASN=4134, Country=CN）**：

| 节点 | Carrier | Country | 自然分 | boost | 实际分 |
|------|---------|---------|--------|-------|--------|
| /电信节点 | 1 | 2 | 3 | - | **3** |
| /全球CDN | 0 | 1 | 1 | 最高=3,不boost | **1** |

**行为**：电信节点 score=3 优先，全球CDN score=1 作为 fallback。boost 未生效，因为最高分=3 >= 3，说明已有完美匹配。

**分析**：国内电信用户走电信专线通常比全球CDN更快，全球CDN 不应抢占流量。

---

### 5.6 国内小运营商用户 + 全球CDN

**配置**：
```yaml
- path: /国内通用
  country_code: ["CN"]
  balance: true

- path: /全球CDN
  balance_universal: true
```

**CN 小运营商用户（Carrier 不匹配任何节点, Country=CN）**：

| 节点 | Carrier | Country | 自然分 | boost | 实际分 |
|------|---------|---------|--------|-------|--------|
| /国内通用 | 0 | 2 | 2 | - | **2** |
| /全球CDN | 0 | 1 | 1 | 最高=2,boost | **2** |

**行为**：国内通用和全球CDN 同分=2，且都是 balance 节点（balance_universal 隐含 balance），balance 组内 >=2 个节点，触发随机选择。

**分析**：对小运营商用户，国内通用节点和全球CDN 的体验可能接近，随机分摊流量合理。

---

### 5.7 全球无匹配用户

**配置**：
```yaml
- path: /电信节点
  asn: [4134]
  country_code: ["CN"]

- path: /境外节点
  country_code_not: ["CN"]

- path: /全球CDN
  balance_universal: true
```

**非洲用户（Carrier 不匹配, Country=NG）**：

| 节点 | Carrier | Country | 自然分 | boost | 实际分 |
|------|---------|---------|--------|-------|--------|
| /电信节点 | 0 | 0 | 0 | - | **0** |
| /境外节点 | 0 | 2 | 2 | - | **2** |
| /全球CDN | 0 | 1 | 1 | 最高=2,boost | **2** |

**行为**：境外节点和全球CDN 同分=2，balance 组随机选择。电信节点 score=0 作为最后 fallback。

---

### 5.8 country_code_not 排除 + balance_universal

**配置**：
```yaml
- path: /CDN-A
  balance_universal: true              # 无配置，Country=1

- path: /CDN-B
  country_code_not: ["CN"]
  balance_universal: true              # CN 用户: Country=0
```

**CN 用户**：

| 节点 | Carrier | Country | 自然分 | boost | 实际分 |
|------|---------|---------|--------|-------|--------|
| /CDN-A | 0 | 1 | 1 | 最高=1,不boost | **1** |
| /CDN-B | 0 | 0 | 0 | Country=0,不boost | **0** |

**行为**：CDN-A 优先，CDN-B 排除 CN 用户后 Country=0，不参与 boost，作为最后 fallback。

**分析**：CDN-B 明确排除了 CN 用户，不应通过 boost 绕过排除意图。

---

### 5.9 只有 1 个 balance 节点

**配置**：
```yaml
- path: /A
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /B
  asn: [4134]
  country_code: ["CN"]
  # 无 balance
```

**CN 电信用户**：

| 节点 | 总分 | balance | 结果 |
|------|------|---------|------|
| /A | 3 | true | balance 组仅 1 个，不随机 |
| /B | 3 | false | 原序 |

**行为**：balance 组内只有 1 个节点（< 2），不进行随机，保持 A → B 顺序。

---

### 5.10 score=0 的 balance 节点

**配置**：
```yaml
- path: /A
  country_code_not: ["CN"]
  balance: true       # CN 用户: score=0

- path: /B
  country_code_not: ["CN"]
  balance: true       # CN 用户: score=0
```

**CN 用户**：

| 节点 | Carrier | Country | 总分 |
|------|---------|---------|------|
| /A | 0 | 0 | **0** |
| /B | 0 | 0 | **0** |

**行为**：A 和 B 同分=0，balance 组随机选择。

**分析**：虽然 A 和 B 明确排除了 CN 用户，但作为最后的 fallback，在所有更优节点都失败时，让它们随机分担流量是合理的。score=0 不意味着"不能用"，只是"不太合适"。

---

## 6. 与原有机制的对比

### 6.1 排序机制对比

| 维度 | 原有机制 | 新机制 |
|------|---------|--------|
| 排序方式 | 多维度淘汰制 | 二维计分制 + tie-breaker |
| 优先级 | ASN > ASO > ISP > CountryCode | Country(2) > Carrier(1)，同分时 ASN > ASO > ISP > CountryCode |
| 同分处理 | 保持原配置顺序 | tie-breaker 后，balance 组随机，non-balance 保持原序 |
| 万能节点 | 无特殊处理 | balance_universal 条件性提升 |

### 6.2 选择行为对比

| 场景 | 原有行为 | 新行为 |
|------|---------|--------|
| 同运营商两个节点 | 永远选第一个 | balance 标记后随机选择 |
| 境外移动子网用户 | 可能选到国内移动节点 | 境外节点优先（Country=2 > Carrier=1） |
| 全球 CDN | 与专用节点同分或更低 | 条件性提升，缺少完美匹配时参与负载均衡 |
| 无配置节点 vs 排除型节点 | 无法区分 | 无配置 Country=1 > 被排除 Country=0 |

### 6.3 向后兼容性

| 配置 | 原有行为 | 新行为 |
|------|---------|--------|
| 无 balance 字段 | 按优先级排序，首个成功返回 | **不变**（balance 默认 false） |
| 无 balance_universal 字段 | 不适用 | **不变**（默认 false） |
| 无 country_code_not 字段 | 不适用 | **不变**（默认空） |

**注意**：当不同节点分别命中 Carrier 和 Country 维度时，可能出现同分情况。为保持与原有淘汰制的兼容性，同分时引入原有优先级作为 tie-breaker：ASN > ASO > ISP > CountryCode。例如：

```yaml
- path: /A
  country_code: ["CN"]        # Carrier=0, Country=2, total=2

- path: /B
  asn: [4134]                 # Carrier=1, Country=1, total=2
```

同分时 tie-breaker：B 命中 ASN，A 未命中 → B 胜出，与旧机制一致。

---

## 7. 配置示例

### 7.1 国内多运营商 + 境外 + 全球CDN

```yaml
- path: /电信A
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /电信B
  asn: [4134]
  country_code: ["CN"]
  balance: true

- path: /联通A
  asn: [4837]
  country_code: ["CN"]
  balance: true

- path: /联通B
  asn: [4837]
  country_code: ["CN"]
  balance: true

- path: /移动
  asn: [9808]
  country_code: ["CN"]

- path: /国内通用
  country_code: ["CN"]

- path: /境外节点
  country_code_not: ["CN"]

- path: /全球CDN
  balance_universal: true
```

**CN 电信用户**：电信A/B 同分=3，balance 组随机 → 联通A/B 同分=2，balance 组随机 → 移动（score=2，non-balance）→ 国内通用（score=2，non-balance）→ 全球CDN（score=1，不boost，最高=3）→ 境外节点（score=0）

**CN 小运营商用户**：移动（score=2，non-balance）、国内通用（score=2，non-balance）、全球CDN（boost到2，balance）同分。balance 组仅全球CDN 1个（< 2），不随机。排列：全球CDN → 移动 → 国内通用。境外节点（score=0）

**境外用户**：境外节点（score=2，non-balance）、全球CDN（boost到2，balance）同分。balance 组仅全球CDN 1个（< 2），不随机。排列：全球CDN → 境外节点。国内通用/电信/联通/移动（score=0）

### 7.2 大文件专用节点

```yaml
- path: /电信高速
  asn: [4134]
  country_code: ["CN"]
  min_size: 100MB
  balance: true

- path: /电信普通
  asn: [4134]
  country_code: ["CN"]
  max_size: 100MB
  balance: true

- path: /全球CDN
  balance_universal: true
```

- 文件 >= 100MB：电信高速参与（电信用户 score=3），全球CDN 不参与（min_size 过滤）
- 文件 < 100MB：电信普通参与（电信用户 score=3），全球CDN 参与（boost 到 2 或保持 1）

---

## 8. 实现要点

### 8.1 数据结构变更

```go
type GslbStorage struct {
    Path            string            `yaml:"path"`
    Aso             []string          `yaml:"aso"`
    Asn             []uint            `yaml:"asn"`
    Isp             []string          `yaml:"isp"`
    CountryCode     []string          `yaml:"country_code"`
    CountryCodeNot  []string          `yaml:"country_code_not"`  // 新增
    Ref             bool              `yaml:"ref"`
    NoDown          bool              `yaml:"no_down"`
    MinSize         SizeSuffix        `yaml:"min_size"`
    MaxSize         SizeSuffix        `yaml:"max_size"`
    Replace         map[string]string `yaml:"replace"`
    Balance         bool              `yaml:"balance"`            // 新增
    BalanceUniversal bool             `yaml:"balance_universal"`  // 新增
}
```

### 8.2 计分函数

```go
func calcCountryScore(s GslbStorage, countryCode string) int {
    if len(s.CountryCodeNot) > 0 {
        for _, cc := range s.CountryCodeNot {
            if cc == countryCode {
                return 0  // 被排除
            }
        }
        return 2  // not 命中
    }
    if len(s.CountryCode) > 0 {
        for _, cc := range s.CountryCode {
            if cc == countryCode {
                return 2  // code 命中
            }
        }
        return 0  // 不匹配
    }
    return 1  // 无配置，通用节点
}

func calcCarrierScore(s GslbStorage, ipinfo IPInfo) int {
    // ASN
    if slices.Contains(s.Asn, ipinfo.Asn) {
        return 1
    }
    // ASO
    if slices.ContainsFunc(s.Aso, func(v string) bool {
        return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(v))
    }) {
        return 1
    }
    // ISP
    if slices.ContainsFunc(s.Isp, func(v string) bool {
        return strings.HasPrefix(strings.ToLower(ipinfo.Isp), strings.ToLower(v))
    }) {
        return 1
    }
    return 0
}
```

### 8.3 排序与随机化

> **注意**：以下为独立函数形式的伪代码。实际代码中排序和随机化逻辑内联在 `Link` 方法中，功能完全一致。

```go
type scoredStorage struct {
    storage   GslbStorage
    score     int
}

func sortAndBalance(storages []scoredStorage, ipinfo IPInfo) []GslbStorage {
    // 按分数降序排序，同分时按原有优先级 tie-breaker
    slices.SortStableFunc(storages, func(a, b scoredStorage) int {
        if a.score != b.score {
            if a.score > b.score { return -1 }
            return 1
        }
        // Tie-breaker: ASN > ASO > ISP > CountryCode
        // ASN
        aAsn := slices.Contains(a.storage.Asn, ipinfo.Asn)
        bAsn := slices.Contains(b.storage.Asn, ipinfo.Asn)
        if aAsn != bAsn {
            if aAsn { return -1 }
            return 1
        }
        // ASO
        aAso := slices.ContainsFunc(a.storage.Aso, func(s string) bool {
            return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(s))
        })
        bAso := slices.ContainsFunc(b.storage.Aso, func(s string) bool {
            return strings.Contains(strings.ToLower(ipinfo.Aso), strings.ToLower(s))
        })
        if aAso != bAso {
            if aAso { return -1 }
            return 1
        }
        // ISP
        aIsp := slices.ContainsFunc(a.storage.Isp, func(s string) bool {
            return strings.HasPrefix(strings.ToLower(ipinfo.Isp), strings.ToLower(s))
        })
        bIsp := slices.ContainsFunc(b.storage.Isp, func(s string) bool {
            return strings.HasPrefix(strings.ToLower(ipinfo.Isp), strings.ToLower(s))
        })
        if aIsp != bIsp {
            if aIsp { return -1 }
            return 1
        }
        // CountryCode
        aCC := slices.Contains(a.storage.CountryCode, ipinfo.CountryCode)
        bCC := slices.Contains(b.storage.CountryCode, ipinfo.CountryCode)
        if aCC != bCC {
            if aCC { return -1 }
            return 1
        }
        return 0
    })

    // 同分组内：balance 节点随机打乱
    result := make([]GslbStorage, 0, len(storages))
    i := 0
    for i < len(storages) {
        score := storages[i].score
        j := i + 1
        for j < len(storages) && storages[j].score == score {
            j++
        }
        // storages[i:j] 是同分组
        group := storages[i:j]

        var balanceGroup []GslbStorage
        var nonBalanceGroup []GslbStorage
        for _, ss := range group {
            if ss.storage.Balance {
                balanceGroup = append(balanceGroup, ss.storage)
            } else {
                nonBalanceGroup = append(nonBalanceGroup, ss.storage)
            }
        }

        // balance 组 >= 2 才随机
        if len(balanceGroup) >= 2 {
            rand.Shuffle(len(balanceGroup), func(a, b int) {
                balanceGroup[a], balanceGroup[b] = balanceGroup[b], balanceGroup[a]
            })
        }

        result = append(result, balanceGroup...)
        result = append(result, nonBalanceGroup...)
        i = j
    }
    return result
}
```

**性能优化建议**：

- 排序比较函数中的 `strings.ToLower()` 会产生字符串分配。建议在配置加载阶段将 ASO/ISP 字段预转为小写缓存，运行时直接比较，避免每次请求重复分配。
- 建议使用 `math/rand/v2`（Go 1.22+）替代 `math/rand`，确保并发安全且无需手动 Seed。

### 8.4 Boost 逻辑

> **注意**：以下为独立函数形式的伪代码。实际代码中 boost 逻辑内联在 `Link` 方法中，功能完全一致。

```go
func applyBoost(storages []scoredStorage, ipinfo IPInfo) {
    // 找到非 universal 节点中的最高分
    maxNonUniversalScore := 0
    for _, ss := range storages {
        if !ss.storage.BalanceUniversal && ss.score > maxNonUniversalScore {
            maxNonUniversalScore = ss.score
        }
    }

    // 对 universal 节点应用 boost
    for i := range storages {
        ss := &storages[i]
        if !ss.storage.BalanceUniversal {
            continue
        }
        countryScore := calcCountryScore(ss.storage, ipinfo.CountryCode)
        if countryScore > 0 && maxNonUniversalScore <= 2 && maxNonUniversalScore > ss.score {
            ss.score = maxNonUniversalScore
        }
    }
}
```

---

## 9. 限制与未来扩展

### 9.1 当前限制

| 限制 | 说明 |
|------|------|
| 均等随机 | 无法控制节点间的流量比例（如 7:3） |
| 无状态 | 每次请求独立，无法实现会话保持 |
| 短期不均匀 | 随机选择在短期内可能分布不均匀 |

### 9.2 未来扩展方向

#### weight 字段（加权随机）

```yaml
- path: /ty/h
  asn: [4134]
  balance: true
  weight: 7        # 70% 流量

- path: /ty/p
  asn: [4134]
  balance: true
  weight: 3        # 30% 流量
```

同分 balance 组内按 weight 加权随机，可精确控制流量比例。

#### 健康检查

定期探测后端节点的可用性和响应时间，自动调整优先级或排除不可用节点，减少用户侧的失败等待时间。

#### 会话保持

基于客户端 IP 的一致性哈希，确保同一用户的连续请求优先走同一节点，减少重复建连开销。
