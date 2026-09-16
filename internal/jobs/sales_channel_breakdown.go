// Package jobs contains concrete job.Job implementations.
package jobs

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hegarty/shop_platform/money"
	"github.com/hegarty/shop_platform/period"

	"github.com/hegarty/shop_analytics/internal/job"
)

const SalesChannelBreakdownName = "sales.channel.breakdown"

// SalesChannelBreakdown groups a tenant's orders by direct shop sales vs.
// Shopify Collective sales, further broken down by Collective partner —
// the platform's first, launch-blocking analytics job.
//
// The "sales" figure used throughout is net_sales (gross - discounts -
// returns, excluding tax and shipping) — see
// shop_docs/docs/database-model.md's metric definitions. Tax is excluded
// deliberately: it's collected on the business's behalf, not revenue.
type SalesChannelBreakdown struct {
	Pool *pgxpool.Pool
}

func (j *SalesChannelBreakdown) Name() string { return SalesChannelBreakdownName }

type row struct {
	channel     string
	partnerName *string
	netSales    money.Amount
	orderCount  int
}

func (j *SalesChannelBreakdown) Execute(ctx context.Context, tenantID string, p period.Range) (*job.Result, error) {
	rows, err := j.Pool.Query(ctx, `
		SELECT
			o.channel,
			cp.name AS partner_name,
			SUM(o.net_sales)::text AS net_sales,
			COUNT(*) AS order_count
		FROM orders o
		LEFT JOIN collective_partners cp ON cp.id = o.collective_partner_id
		WHERE o.tenant_id = $1
		  AND o.created_at >= $2
		  AND o.created_at <  $3
		GROUP BY o.channel, cp.name
	`, tenantID, p.Start, p.End)
	if err != nil {
		return nil, fmt.Errorf("sales.channel.breakdown: query: %w", err)
	}
	defer rows.Close()

	var groups []row
	for rows.Next() {
		var r row
		var netSalesStr string
		if err := rows.Scan(&r.channel, &r.partnerName, &netSalesStr, &r.orderCount); err != nil {
			return nil, fmt.Errorf("sales.channel.breakdown: scan: %w", err)
		}
		amt, err := money.FromDecimalString(netSalesStr)
		if err != nil {
			return nil, fmt.Errorf("sales.channel.breakdown: parse net_sales: %w", err)
		}
		r.netSales = amt
		groups = append(groups, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sales.channel.breakdown: rows: %w", err)
	}

	sales := aggregate(groups)

	return &job.Result{
		TenantID: tenantID,
		Job:      SalesChannelBreakdownName,
		Period: job.ResultPeriod{
			Start: p.Start.Format("2006-01-02T15:04:05Z07:00"),
			End:   p.End.Format("2006-01-02T15:04:05Z07:00"),
			Label: p.Label,
		},
		Sales: sales,
	}, nil
}

func aggregate(groups []row) *job.SalesResult {
	var shopTotal, collectiveTotal money.Amount
	var orderCount int
	partners := map[string]money.Amount{}

	for _, g := range groups {
		orderCount += g.orderCount
		switch g.channel {
		case "shop":
			shopTotal = shopTotal.Add(g.netSales)
		case "collective":
			collectiveTotal = collectiveTotal.Add(g.netSales)
			if g.partnerName != nil {
				partners[*g.partnerName] = partners[*g.partnerName].Add(g.netSales)
			}
		}
	}

	partnerList := make([]job.PartnerBreakdown, 0, len(partners))
	for name, amt := range partners {
		partnerList = append(partnerList, job.PartnerBreakdown{Name: name, Sales: amt})
	}
	sort.Slice(partnerList, func(i, j int) bool { return partnerList[i].Sales > partnerList[j].Sales })

	total := shopTotal.Add(collectiveTotal)
	var aov money.Amount
	if orderCount > 0 {
		// Round to nearest cent rather than truncate.
		aov = money.Amount((int64(total) + int64(orderCount)/2) / int64(orderCount))
	}

	return &job.SalesResult{
		Total:      total,
		Shop:       shopTotal,
		OrderCount: orderCount,
		AOV:        aov,
		Collective: job.CollectiveBreakdown{
			Total:    collectiveTotal,
			Partners: partnerList,
		},
	}
}
