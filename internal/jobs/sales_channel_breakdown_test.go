package jobs

import (
	"testing"

	"github.com/hegarty/shop_platform/money"
)

func amt(s string) money.Amount {
	a, err := money.FromDecimalString(s)
	if err != nil {
		panic(err)
	}
	return a
}

func strp(s string) *string { return &s }

func TestAggregate_MatchesWorkedExample(t *testing.T) {
	// Mirrors the example in the original product brief:
	// Shop $6,842 / Collective $3,412 (E-Moto Co $1,282, Ride Electric $934,
	// XYZ Motors $721, ABC Bikes $475) / 45 orders / total $10,254.
	groups := []row{
		{channel: "shop", netSales: amt("6842.00"), orderCount: 30},
		{channel: "collective", partnerName: strp("E-Moto Co"), netSales: amt("1282.00"), orderCount: 5},
		{channel: "collective", partnerName: strp("Ride Electric"), netSales: amt("934.00"), orderCount: 4},
		{channel: "collective", partnerName: strp("XYZ Motors"), netSales: amt("721.00"), orderCount: 3},
		{channel: "collective", partnerName: strp("ABC Bikes"), netSales: amt("475.00"), orderCount: 3},
	}

	result := aggregate(groups)

	if result.Total != amt("10254.00") {
		t.Errorf("Total = %s, want 10254.00", result.Total.DecimalString())
	}
	if result.Shop != amt("6842.00") {
		t.Errorf("Shop = %s, want 6842.00", result.Shop.DecimalString())
	}
	if result.Collective.Total != amt("3412.00") {
		t.Errorf("Collective.Total = %s, want 3412.00", result.Collective.Total.DecimalString())
	}
	if result.OrderCount != 45 {
		t.Errorf("OrderCount = %d, want 45", result.OrderCount)
	}
	// $10254.00 / 45 = $227.8666... -> rounds to $227.87, matching the brief.
	if result.AOV != amt("227.87") {
		t.Errorf("AOV = %s, want 227.87", result.AOV.DecimalString())
	}
	if len(result.Collective.Partners) != 4 {
		t.Fatalf("expected 4 partners, got %d", len(result.Collective.Partners))
	}
	if result.Collective.Partners[0].Name != "E-Moto Co" {
		t.Errorf("top partner = %q, want E-Moto Co (sorted descending by sales)", result.Collective.Partners[0].Name)
	}
}

func TestAggregate_NoOrders(t *testing.T) {
	result := aggregate(nil)
	if result.Total != 0 || result.OrderCount != 0 || result.AOV != 0 {
		t.Errorf("expected all-zero result for no orders, got %+v", result)
	}
}

func TestAggregate_CollectiveWithUnknownPartner(t *testing.T) {
	// channel=collective but partner_name is NULL (ambiguous classification
	// — see normalize.ClassifyChannel) must still count toward Collective's
	// total without appearing in the partner breakdown.
	groups := []row{
		{channel: "collective", partnerName: nil, netSales: amt("100.00"), orderCount: 1},
	}
	result := aggregate(groups)

	if result.Collective.Total != amt("100.00") {
		t.Errorf("Collective.Total = %s, want 100.00", result.Collective.Total.DecimalString())
	}
	if len(result.Collective.Partners) != 0 {
		t.Errorf("expected no partner breakdown entries for unattributed sales, got %+v", result.Collective.Partners)
	}
}
