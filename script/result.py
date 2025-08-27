import matplotlib.pyplot as plt

# Data parsing
round_data = []
with open('data/result.txt', 'r') as f:
    for line in f:
        if not line.strip(): continue
        parts = line.strip().split()
        round_num = int(parts[0])
        values = parts[1:]
        round_data.append((round_num, values))

# Sort rounds chronologically
round_data.sort(key=lambda x: x[0])
all_rounds = sorted({r[0] for r in round_data})

# Track value presence
value_records = {}
for rnd, values in round_data:
    for val in values:
        if val not in value_records:
            value_records[val] = {
                'first_round': rnd,
                'present_rounds': set()
            }
        value_records[val]['present_rounds'].add(rnd)

# Sort values by first appearance
sorted_values = sorted(
    value_records.keys(),
    key=lambda x: (value_records[x]['first_round'], x)
)

# Visualization setup
plt.figure(figsize=(14, 8), dpi=100)
colors = plt.cm.tab20.colors

# Plot value persistence
for idx, value in enumerate(sorted_values):
    first = value_records[value]['first_round']
    presence = value_records[value]['present_rounds']

    # Draw continuous presence
    start = first
    for rnd in sorted(all_rounds):
        if rnd >= first:
            if rnd in presence:
                end = rnd
            else:
                plt.hlines(
                    y=idx, xmin=start, xmax=end,
                    colors=colors[idx%20], lw=3
                )
                start = rnd + 1

    # Final segment
    plt.hlines(
        y=idx, xmin=start, xmax=max(all_rounds),
        colors=colors[idx%20], lw=3
    )

    # Mark first appearance
    plt.plot(first, idx, 'o',
            markersize=8,
            color=colors[idx%20],
            markeredgecolor='black')

# Axis formatting
plt.yticks(
    ticks=range(len(sorted_values)),
    labels=[f"{value} (First@R{value_records[value]['first_round']})"
           for value in sorted_values],
    fontsize=10
)
plt.xticks(
    ticks=all_rounds,
    labels=[f"R{rnd}" for rnd in all_rounds],
    rotation=90,
    fontsize=7
)

# Labels and title
plt.grid(axis='x', linestyle=':', alpha=0.5)
plt.xlabel("Round Number", fontsize=12)
plt.title("Value Persistence Across Rounds", fontsize=14, pad=20)
plt.margins(y=0.1)

# Save output
plt.tight_layout()
plt.savefig('data/value_persistence.png', bbox_inches='tight')
plt.close()

print("Chart saved as value_persistence.png")