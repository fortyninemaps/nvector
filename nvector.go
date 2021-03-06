/*
	Implements the "n-vector" based geodetic operations from

		Gade, Kenneth, "A Non-Singular Horizontal Position Representation",
		The Journal of Navigation (2010), 63, 395-417.
		doi:10.1017/S0373463309990415

*/
package nvector

import (
	"fmt"
	"math"
)

type ILonLat interface {
	ToNVector() INVector
}

type INVector interface {
	ToLonLat() ILonLat
	Magnitude() float64
}

type IPVector interface {
	ToNVector() INVector
	Magnitude() float64
}

type Vec3 [3]float64

type Matrix3 [3][3]float64

type NVector struct {
	Vec3
}

type PVector struct {
	Vec3
}

type LonLat struct {
	Lon float64
	Lat float64
}

// Ellipsoid represents a geographical ellipsoid in terms of its major and
// minor axes
type Ellipsoid struct {
	a, b float64
}

type InvalidLatitudeError struct {
	Lat float64
}

func (e InvalidLatitudeError) Error() string {
	return fmt.Sprintf("invalid latitude: %f", e.Lat)
}

type NoIntersectionError struct {
}

func (e NoIntersectionError) Error() string {
	return fmt.Sprintf("no intersection")
}

func cross(u, v *Vec3) *Vec3 {
	return &Vec3{u[1]*v[2] - u[2]*v[1], u[2]*v[0] - u[0]*v[2], u[0]*v[1] - u[1]*v[0]}
}

func dot(u, v *Vec3) float64 {
	return u[0]*v[0] + u[1]*v[1] + u[2]*v[2]
}

func (v *Vec3) Magnitude() float64 {
	return math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
}

func (m *Matrix3) Mult(v *Vec3) Vec3 {
	var p Vec3
	p[0] = v[0]*m[0][0] + v[1]*m[0][1] + v[2]*m[0][2]
	p[1] = v[0]*m[1][0] + v[1]*m[1][1] + v[2]*m[1][2]
	p[2] = v[0]*m[2][0] + v[1]*m[2][1] + v[2]*m[2][2]
	return p
}

func (m *Matrix3) Transpose() Matrix3 {
	var tr Matrix3
	tr[0] = [3]float64{m[0][0], m[1][0], m[2][0]}
	tr[1] = [3]float64{m[0][1], m[1][1], m[2][1]}
	tr[2] = [3]float64{m[0][2], m[1][2], m[2][2]}
	return tr
}

func NewLonLat(londeg float64, latdeg float64) (*LonLat, error) {
	lon := londeg * math.Pi / 180.0
	lat := latdeg * math.Pi / 180.0
	lon = math.Mod((lon+math.Pi), 2*math.Pi) - math.Pi
	if (lat < -0.5*math.Pi) || (lat > 0.5*math.Pi) {
		lonlat := new(LonLat)
		return lonlat, InvalidLatitudeError{latdeg}
	}
	return &LonLat{lon, lat}, nil
}

// ToNVector returns a Cartesian position vector.
func (ll *LonLat) ToNVector() NVector {
	z := math.Sin(ll.Lat)
	y := math.Sin(ll.Lon) * math.Cos(ll.Lat)
	x := math.Cos(ll.Lon) * math.Cos(ll.Lat)
	return NVector{Vec3{x, y, z}}
}

func (ll *LonLat) String() string {
	londeg := ll.Lon * 180.0 / math.Pi
	latdeg := ll.Lat * 180.0 / math.Pi
	return fmt.Sprintf("(%.6f, %.6f)", londeg, latdeg)
}

// ToLonLat returns a LonLat struct, where lon: [-pi, pi) and lat: [-pi/2, pi/2].
func (nv *NVector) ToLonLat() LonLat {
	lat := math.Atan2(nv.Vec3[2], math.Sqrt(nv.Vec3[0]*nv.Vec3[0]+nv.Vec3[1]*nv.Vec3[1]))
	lon := math.Atan2(nv.Vec3[1], nv.Vec3[0])
	lon = math.Mod((lon+0.5*math.Pi), math.Pi) - 0.5*math.Pi
	return LonLat{lon, lat}
}

// ToPVector returns a surface-normal vector, given an ellipsoid.
func (nv *NVector) ToPVector(ellps *Ellipsoid) PVector {
	absq := ellps.a * ellps.a / (ellps.b * ellps.b)
	coeff := ellps.b / math.Sqrt(nv.Vec3[2]*nv.Vec3[2]+
		absq*nv.Vec3[1]*nv.Vec3[1]+
		absq*nv.Vec3[0]*nv.Vec3[0])
	return PVector{Vec3{coeff * absq * nv.Vec3[0], coeff * absq * nv.Vec3[1], coeff * nv.Vec3[2]}}
}

// ToNVector returns a Cartesian position vector, given an ellipsoid.
func (pv *PVector) ToNVector(ellps *Ellipsoid) NVector {
	eccen := math.Sqrt(1 - ellps.b*ellps.b/(ellps.a*ellps.a))
	eccen2 := eccen * eccen
	eccen4 := eccen2 * eccen2
	a2 := ellps.a * ellps.a
	q := (1 - eccen2) / a2 * pv.Vec3[2] * pv.Vec3[2]
	p := (pv.Vec3[1]*pv.Vec3[1] + pv.Vec3[0]*pv.Vec3[0]) / a2
	r := (p + q - eccen4) / 6.0
	s := eccen4 * p * q / (4 * math.Pow(r, 3))
	t := math.Cbrt(1 + s + math.Sqrt(s*(2+s)))
	u := r * (1 + t + 1.0/t)
	v := math.Sqrt(u*u + eccen4*q)
	w := 0.5 * eccen2 * (u + v - q) / v
	k := math.Sqrt(u+v+w*w) - w
	d := k * math.Sqrt(pv.Vec3[1]*pv.Vec3[1]+pv.Vec3[0]*pv.Vec3[0]) / (k + eccen2)
	coeff := 1.0 / math.Sqrt(d*d+pv.Vec3[2]*pv.Vec3[2])
	kke2 := k / (k + eccen2)
	return NVector{
		Vec3{-coeff * kke2 * pv.Vec3[0],
			-coeff * kke2 * pv.Vec3[1],
			coeff * pv.Vec3[2]}}
}

// RotationMatrix returns the 3x3 matrix relating the Earth-centered
// non-singular coordinate frame to the North-East-Down singular coordinate
// frame.
func (nv *NVector) RotationMatrix() Matrix3 {
	east := cross(&Vec3{0, 0, 1}, &nv.Vec3)
	north := cross(&nv.Vec3, east)

	a := north[0] / north.Magnitude()
	b := east[0] / east.Magnitude()
	c := -nv.Vec3[0]
	d := north[1] / north.Magnitude()
	e := east[1] / east.Magnitude()
	f := -nv.Vec3[1]
	g := north[2] / north.Magnitude()
	h := east[2] / east.Magnitude()
	i := -nv.Vec3[2]
	return Matrix3{[3]float64{a, b, c}, [3]float64{d, e, f}, [3]float64{g, h, i}}
}

// SphericalDistance returns the distance from another NVector on a sphere with
// radius *R*
func (nv *NVector) SphericalDistance(nv2 *NVector, R float64) float64 {
	s_ab := math.Atan2(cross(&nv.Vec3, &nv2.Vec3).Magnitude(),
		dot(&nv.Vec3, &nv2.Vec3)) * R
	return s_ab
}

// Azimuth returns the azimuth and back azimuth from one NVector to another
// along an ellipse
func (nv *NVector) Azimuth(nv2 *NVector, ellps *Ellipsoid) float64 {
	pv1 := nv.ToPVector(ellps)
	pv2 := nv2.ToPVector(ellps)
	delta_E := Vec3{pv2.Vec3[0] - pv1.Vec3[0], pv2.Vec3[1] - pv1.Vec3[1], pv2.Vec3[2] - pv1.Vec3[2]}

	rotMat_EN := nv.RotationMatrix()
	rotMat_NE := rotMat_EN.Transpose()
	delta_N := rotMat_NE.Mult(&delta_E)
	return math.Atan2(delta_N[1], delta_N[0])
}

// Forward returns the NVector position arrived at by moving in an azimuthal
// direction for a given distance along an ellipse
func (nv *NVector) Forward(az, distance, radius float64) NVector {
	east := cross(&nv.Vec3, &Vec3{0, 0, -1})
	north := cross(&nv.Vec3, east)

	cos_az := math.Cos(az)
	sin_az := math.Sin(az)
	vec_az := Vec3{north[0]*cos_az + east[0]*sin_az,
		north[1]*cos_az + east[1]*sin_az,
		north[2]*cos_az + east[2]*sin_az}

	// Great circle angle travelled
	sab := distance / radius
	cos_sab := math.Cos(sab)
	sin_sab := math.Sin(sab)
	resultant := Vec3{nv.Vec3[0]*cos_sab + vec_az[0]*sin_sab,
		nv.Vec3[1]*cos_sab + vec_az[1]*sin_sab,
		nv.Vec3[2]*cos_sab + vec_az[2]*sin_sab}
	return NVector{resultant}
}

func interpLinear(x, x0, x1, y0, y1 float64) float64 {
	return (x-x0)/(x1-x0)*(y1-y0) + y0
}

// Interpolate returns the NVector representing the intermediate position
// between two other NVectors. *frac* is the fractional distance between *nv*
// and *nv2*.
func (nv *NVector) Interpolate(nv2 *NVector, frac float64) NVector {
	result := new(NVector)
	result.Vec3[0] = interpLinear(frac, 0, 1, nv.Vec3[0], nv2.Vec3[0])
	result.Vec3[1] = interpLinear(frac, 0, 1, nv.Vec3[1], nv2.Vec3[1])
	result.Vec3[2] = interpLinear(frac, 0, 1, nv.Vec3[2], nv2.Vec3[2])
	return *result
}

// Intersection returns the spheroidal intersection point between two geodesics
// defined by an NVector pair, if it exists. If no intersection exists,
// NoIntersectionError is returned
func Intersection(nv1a, nv1b, nv2a, nv2b *NVector) (NVector, error) {
	var normalA, normalB, intersection *Vec3
	var err error

	normalA = cross(&nv1a.Vec3, &nv1b.Vec3)
	normalB = cross(&nv2a.Vec3, &nv2b.Vec3)
	intersection = cross(normalA, normalB)

	// Select the intersection on the right side of the spheroid
	if dot(intersection, &nv1a.Vec3) < 0 {
		intersection[0] = -intersection[0]
		intersection[1] = -intersection[1]
		intersection[2] = -intersection[2]
	}

	result := NVector{*intersection}

	// Tests whether intersection is between segment endpoints to within ~4cm
	var dab, dai, dbi float64
	dab = nv1a.SphericalDistance(nv1b, 1.0)
	dai = nv1a.SphericalDistance(&result, 1.0)
	dbi = nv1b.SphericalDistance(&result, 1.0)

	if math.Abs(dab-dai-dbi) > 1e-9 {
		err = NoIntersectionError{}
	}

	dab = nv2a.SphericalDistance(nv2b, 1.0)
	dai = nv2a.SphericalDistance(&result, 1.0)
	dbi = nv2b.SphericalDistance(&result, 1.0)

	if math.Abs(dab-dai-dbi) > 1e-9 {
		err = NoIntersectionError{}
	}

	return NVector{*intersection}, err
}
