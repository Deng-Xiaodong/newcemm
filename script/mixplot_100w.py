import matplotlib.pyplot as plt

#plt.rcParams['font.sans-serif'] = ['SimHei'] # 指定默认字体（解决中文无法显示的问题）
# 读取两个文件的数据
with open('data/cost_100w_100w.txt', 'r') as f:
    data_100w = [int(line.strip()) for line in f if line.strip()]
with open('data/cost_1000w_100w.txt', 'r') as f:
    data_1000w = [int(line.strip()) for line in f if line.strip()]

# 验证数据一致性
assert len(data_100w) == len(data_1000w), "错误：文件行数不一致"

# 单位转换 (微秒→毫秒)
y_100w = [t/1000 for t in data_100w]
y_1000w = [t/1000 for t in data_1000w]

# 生成横坐标 (从0开始，增量5的刻度)
x_points = list(range(1, len(data_100w)+1))  # 实际数据点位置
x_ticks = list(range(0, len(data_100w)+1, 5))  # 显示刻度位置
y_ticks = list(range(0, int(max(y_1000w))+1, 20))  # 显示刻度位置

# 创建图形
plt.figure(figsize=(12, 6), dpi=150)
plt.plot(x_points, y_100w,
         color='#1f77b4',
         marker='o',
         linestyle='-',
         linewidth=1.5,
         label='Million level database')
plt.plot(x_points, y_1000w,
         color='#ff7f0e',
         marker='s',
         linestyle='--',
         linewidth=1.5,
         label='Ten million level database')

# 坐标轴设置
plt.xticks(x_ticks, rotation=45)
plt.yticks(y_ticks, rotation=45)
plt.xlabel("Dummy Quantity (×10000)", fontsize=12)
plt.ylabel("Time (milliseconds)", fontsize=12)
plt.title("Time Measurement vs Dummy Quantity", fontsize=14, pad=20)

# 增强可视化
plt.grid(True, linestyle=':', alpha=0.6)
plt.legend(loc='upper left')
plt.tight_layout()

# 保存文件
plt.savefig('data/database_comparison_100w.png', bbox_inches='tight')
plt.close()

print("图表已保存至当前目录：database_comparison.png")