package main

import (
	"reflect"
	"testing"

	"github.com/ariefusry/diecast-auction/internal/service"
)

// Fixtures are trimmed copies of real pages, one per wiki layout.
const hotWheelsPage = `{{casting
|name = Nissan Skyline GT-R (BNR34)
|series = [[2010 New Models]]
|years = [[2010]] - Present
|image = R34Sketch.png}}

==Versions==
{| class="wikitable sortable" style="font-size: 85%;" width="100%"
|-
! style="border-style: none none solid solid" |'''Col #'''
! style="border-style: none none solid solid" |'''Year'''
! style="border-style: none none solid solid" |'''Series'''
! style="border-style: none none solid solid" |'''Color'''
! style="border-style: none none solid solid" |'''Photo'''
|-
|007/214
|[[List of 2010 Hot Wheels (International)|2010]]
|[[2010 HW Premiere]]<br>7/52
|Metalflake Dark Blue
|[[File:R34.JPG|100px]]
|}`

const miniGTPage = `== Variation ==

=== Millenium Jade ===
{| class="fandom-table"
! rowspan="2" |No.
!PKG. Name
!Livery
!Prod. Yr
|-
! colspan="3" |Note
|-
! colspan="4" |Image
|-
| rowspan="2" |1003
|Nissan Skyline GT-R (R34) V-Spec II Nür
|Millenium Jade
|2025
|-
| colspan="3" |
* Left-hand drive (LHD) version
|-
| colspan="4" |<gallery>
File:MGT1003 A.jpg
</gallery>
|}`

const inno64Page = `{{Casting|image=Honda_NSXR_NA2_Championship_White_Front.webp|name=Honda NSX-R (NA2)|year=2018|years=2018 -|Manufacturer=Honda}}
== Versions ==
{| class="wikitable" style="width: 100%;"
|+
| style="border-style: none none solid solid" |'''Date'''
| style="border-style: none none solid solid" |'''SKU'''
| style="border-style: none none solid solid" |'''Color'''
| style="border-style: none none solid solid" |'''Description'''
|-
| align="center" |November 2021
| align="center" |IN64-NSX2-WHI
| align="center" |Championship White
| align="center" |Honda NSX-R (NA2) Coupe 1992
"Team 5ZIGEN"
|}`

// Tarmac Works pages are titled by SKU; the model and color share the Description cell.
const tarmacPage = `{| class="wikitable" style="width: 100%;""
|+
|bgcolor="#7b8b63" align="center"|<font color="#ffffff"> '''Series'''
|bgcolor="#7b8b63" align="center"|<font color="#ffffff"> '''SKU'''
|bgcolor="#7b8b63" align="center"|<font color="#ffffff"> '''Date'''
|bgcolor="#7b8b63" align="center"|<font color="#ffffff"> '''Description'''
|-
|align="center"|[[HOBBY64]]
|align="center"|T64-006-BB
|align="center"|[[2017|September 2017]] <br/> Release 2
|align="center"|[[Mercedes-AMG GTR]] <br/> Brilliant Blue Metallic
|}
<div>[[File:T64-006-BB front.jpg|363x363px]]</div>`

// Tomica pages have no infobox or versions table; the year is only in prose.
const tomicaPage = `'''Tomica No. 76 Honda Civic Type R''' is a toy released within the classic [[Tomica]] toyline.

==Overview==
The Honda Civic Type R was released July 16, 2016, and retired May, 2018. It replaced the [[No. 76 Isuzu Giga Dump Truck]].`

func TestParsePage(t *testing.T) {
	cases := []struct {
		name  string
		brand string
		title string
		text  string
		want  service.Item
	}{
		{"hot wheels", "Hot Wheels", "Nissan Skyline GT-R (BNR34)", hotWheelsPage, service.Item{
			Brand: "Hot Wheels", ModelName: "Nissan Skyline GT-R (BNR34)", Series: "2010 New Models",
			Year: 2010, Color: "Metalflake Dark Blue", Tags: []string{"nissan", "jdm"},
		}},
		{"mini gt", "Mini GT", "1999 Nissan Skyline GT-R (R34)", miniGTPage, service.Item{
			Brand: "Mini GT", ModelName: "1999 Nissan Skyline GT-R (R34)",
			Year: 2025, Color: "Millenium Jade", Tags: []string{"nissan", "jdm"},
		}},
		{"inno64", "Inno64", "Honda NSX-R (NA2)", inno64Page, service.Item{
			Brand: "Inno64", ModelName: "Honda NSX-R (NA2)",
			Year: 2018, Color: "Championship White", Tags: []string{"honda", "jdm"},
		}},
		{"tomica prose", "Tomica", "No. 76 Honda Civic Type R", tomicaPage, service.Item{
			Brand: "Tomica", ModelName: "No. 76 Honda Civic Type R", Year: 2016, Tags: []string{"honda", "jdm"},
		}},
		{"tarmac works sku page", "Tarmac Works", "T64-006-BB", tarmacPage, service.Item{
			Brand: "Tarmac Works", ModelName: "Mercedes-AMG GTR", Series: "HOBBY64",
			Year: 2017, Color: "Brilliant Blue Metallic", Tags: []string{"mercedes-benz"},
		}},
		{"photos only, color from title", "Mini GT", "1/64 McLaren Senna Victory Grey", "[[File:MGT1.jpg|thumb]]", service.Item{
			Brand: "Mini GT", ModelName: "McLaren Senna Victory Grey", Color: "Grey", Tags: []string{"mclaren"},
		}},
		{"tuner prefix", "Mini GT", "HKS Toyota GR Supra ADVAN 2020", "", service.Item{
			Brand: "Mini GT", ModelName: "HKS Toyota GR Supra ADVAN 2020", Tags: []string{"toyota", "jdm"},
		}},
		{"short year prefix", "Hot Wheels", "'90 Honda Civic EF", "", service.Item{
			Brand: "Hot Wheels", ModelName: "'90 Honda Civic EF", Tags: []string{"honda", "jdm"},
		}},
		{"model without make", "Hot Wheels", "1972 Skyline H/T 2000GT-R", "", service.Item{
			Brand: "Hot Wheels", ModelName: "1972 Skyline H/T 2000GT-R", Tags: []string{"nissan", "jdm"},
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parsePage(c.brand, c.title, c.text); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got  %+v\nwant %+v", got, c.want)
			}
		})
	}
}

func TestPickSpreadsEvenly(t *testing.T) {
	got := pick([]string{"a", "b", "c", "d", "e", "f"}, 3)
	if !reflect.DeepEqual(got, []string{"a", "c", "e"}) {
		t.Fatalf("pick = %v", got)
	}
	if got := pick([]string{"a"}, 3); len(got) != 1 {
		t.Fatalf("pick short = %v", got)
	}
}

func TestIsCasting(t *testing.T) {
	for title, want := range map[string]bool{
		"Nissan Skyline GT-R (BNR34)": true,
		"'90 Honda Civic EF":          true,
		"1999 Nissan Skyline (1:20)":  false, // another scale
		"Mazda":                       false, // a make's index page
		"Mitsubishi":                  false,
		"T64-006-BB":                  true,
		"T43-008-08HEC96 (Cancelled)": false, // Tarmac Works 1:43, never released
	} {
		if got := isCasting(title); got != want {
			t.Errorf("isCasting(%q) = %v, want %v", title, got, want)
		}
	}
}
