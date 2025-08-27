import matplotlib.pyplot as plt

# 读取数据文件
with open('data/cost_100w.txt', 'r') as f:
    microseconds = [int(line.strip()) for line in f if line.strip()]

# 生成坐标数据
x = [i  for i in range(len(microseconds))]  # 横坐标：dummy数量（步长1000）
y = [t / 1000 for t in microseconds]             # 纵坐标：转换为毫秒

# 创建图形
plt.figure(figsize=(12, 6), dpi=150)
plt.plot(x, y, 'b-', linewidth=2.5, marker='o', markersize=4)

# 设置坐标轴标签和标题
plt.xlabel('Dummy Quantity (×10000)', fontsize=12)
plt.ylabel('Time (milliseconds)', fontsize=12)
plt.title('Time Measurement vs Dummy Quantity', fontsize=14)

# 设置网格和刻度格式
plt.grid(True, linestyle='--', alpha=0.7)
plt.xticks(range(0, max(x)+2, 5), rotation=300)
plt.yticks(range(0, int(max(y))+2, 10))

# 优化布局并保存为图片
plt.tight_layout()
plt.savefig('data/time_vs_dummy.png')  # 默认保存为PNG格式
plt.close()  # 关闭图形释放内存

print("图表已保存为 time_vs_dummy.png")