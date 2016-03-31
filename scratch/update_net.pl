#!/usr/bin/perl

use strict; 
use XML::Simple; 
use Data::Dumper; 
use Data::Validate::IP qw (is_private_ipv4) ; 
use Net::DNS ; 
use Net::Ping ; 
use YAML::Syck; 
   

my $CLOUD_INIT_DEF = '/etc/default/cloud-init'; 
my $FQDN  = 'phonepe.int' ;
my $PMCDIR = '/var/local/pmc'; 
my $GATEWAY = '192.168.200.1' ; 
my $HOST_IMAGE_LOCATION = '/var/lib/libvirt/images'; 
my $ORIGINAL_SOURCE_IMAGE = '/var/lib/libvirt/images/trusty.ORIG.img'; 
my $LIBVIRT_XML_TEMPLATE = '/etc/default/PMCLibvirtTemplate.xml' ; 
my $CLOUD_LOCAL_DS = '/usr/bin/cloud-localds'; 

my $hostname = shift; 
my $ip = shift; 
my $size = shift; 
$size = 'C1M2' if(!defined $size) ; 
my $sizef = VerifyValidSize($size) ; 

die __FILE__." <hostname> <ip>\n" if !(defined $ip && defined $hostname) ; 


my $cmd = `virsh net-dumpxml default`; 
#print Dumper $cmd ; 
my $xml_ref = XMLin( $cmd, KeepRoot => 1, ForceArray => 1,);


my $ipf = VerifyValidIP($ip,$xml_ref->{network}->[0]->{ip}->[0]->{dhcp}->[0]->{host}) ;
die if(!$ipf) ;

my $hostnamef = VerifyValidHost($hostname,$xml_ref->{network}->[0]->{ip}->[0]->{dhcp}->[0]->{host}) ; 
die if(!$hostnamef) ;

die if(!$sizef) ; 



GenerateNetworkConfig($hostname,$ip,$PMCDIR.'/default.xml') ; 
GenerateCloudInitConfig($hostname) ; 
GenerateLibvirtXML($hostname,$ip,$size) ; 
DefineAndStartVM($hostname) ; 

sub DefineAndStartVM {
	my $hn = shift ; 
	my $dnsf = `virsh define ${PMCDIR}/${hn}.xml && virsh start $hn`; 
	print $dnsf; 
}

sub VerifyValidSize {
	my $s = shift; 
	my %approved;
	$approved{C} = 'CPU'; 
	$approved{M} = 'MEMORY'; 
	$approved{D} = 'DISK'; 
	my @fields = ($s =~ /([a-z]\d+)/gi) ; 
	foreach (@fields) {
		my ($k,$v) = ($1,$2) if(/^([a-z])(\d+)$/i) ;
		return 0 if(!defined $approved{uc($k)}) ; 
	}
	return 1;
}
sub GetMemory {
	my $sz = shift ; 
	my $mem = $1 if($sz =~ /M(\d+)/) ; 
	return $mem * 1024 * 1024 ; 
}
sub GetCPU {
	my $sz = shift ; 
	my $cpu = $1 if($sz =~ /C(\d+)/) ; 
	return $cpu; 
}
sub GetImageLocation {
	my $hn = shift; 
	my $image_loc = $HOST_IMAGE_LOCATION."/${hn}.img" ; 
	my $cp_st = system("cp",$ORIGINAL_SOURCE_IMAGE,$image_loc); 
	if(!$cp_st) {
		return $image_loc; 
	}
	die "Copy of $ORIGINAL_SOURCE_IMAGE -> $image_loc FAILED\n"; 
}

sub GenerateLibvirtXML{
	my $hn = shift ;
	my $ip = shift; 
	my $sz = shift; 
	my %ret; 
	$ret{PMC__HOSTNAME} = $hn;
	$ret{PMC__UUID} = GetUUID();
	$ret{PMC__MEMORY} = GetMemory($sz) ; 
	$ret{PMC__VCPU} = GetCPU($sz) ; 
	$ret{PMC__KERNEL} = $PMCDIR.'/trusty-server-cloudimg-amd64-vmlinuz-generic' ;
	$ret{PMC__CLOUDINITIMAGE} = "${PMCDIR}/hostdata.${hn}.img";
	$ret{PMC__MAC} = GenMAC($ip) ;
	$ret{PMC__IP} = $ip;
	$ret{PMC__GATEWAY} = $GATEWAY; 
	$ret{PMC__HOSTIMAGE} = GetImageLocation($hn) ; 
	open F,$LIBVIRT_XML_TEMPLATE ; 
	my @xml = <F> ;
	close F; 
	my $xml_str = join("",@xml) ; 
	foreach my $k (keys %ret) {
		$xml_str =~ s/__${k}__/$ret{$k}/g ; 
	}
	open F,"> ${PMCDIR}/${hn}.xml"; 
	print F $xml_str;
	close F; 

}
sub GetUUID {
	my $uuid = `uuidgen`; 
	chomp $uuid; 
	return $uuid; 
}

sub GenerateCloudInitConfig {
	my %ret; 
	my $hn = shift; 
	my $def = LoadFile($CLOUD_INIT_DEF) ; 
	#my $def = LoadFile("/home/user/my-user-data") ; 
	$def->{hostname} = $hn; 
	$def->{fqdn} = join(".",$hn,$FQDN) ; 
	#print Dumper $def; 
	my $yml = Dump($def) ; 
	$yml =~ s/^---/#cloud-config/; 
	open F,"> ${PMCDIR}/hostdata.${hn}" ;
	print F $yml;
	close F; 
	my $cloud_local_ds_f = system($CLOUD_LOCAL_DS,"${PMCDIR}/hostdata.${hn}.img","${PMCDIR}/hostdata.${hn}") ; 
	print "system($CLOUD_LOCAL_DS ${PMCDIR}/hostdata.${hn}.img ${PMCDIR}/hostdata.${hn}\n"; 
	print "STATUS=$cloud_local_ds_f\n"; 
	die if( $cloud_local_ds_f ) ; 
}


sub GenerateNetworkConfig { 
	my $hn =  shift; 
	my $ip = shift ; 
	my $f = shift ; 
	my $mac = GenMAC($ip) ; 
	$xml_ref->{network}->[0]->{ip}->[0]->{dhcp}->[0]->{host}->{$hostname}{ip} = $ip; 
	$xml_ref->{network}->[0]->{ip}->[0]->{dhcp}->[0]->{host}->{$hostname}{mac} = $mac; 
	XMLout( $xml_ref, KeepRoot => 1, NoAttr => 0, OutputFile => "/tmp/default.xml");
	my $net_start = `virsh net-define /tmp/default.xml && virsh net-destroy default && virsh net-start default` ; 
	print "NETSTART\n$net_start\n\n"; 
} 


sub GenMAC {
	my $ip = shift ; 
	my @octets = split/\./,$ip ; 
	my $mac = sprintf("52:54:00:%x:%x:%x",$octets[1],$octets[2],$octets[3]);
	return $mac ; 
}

sub VerifyValidIP {
	my $ip = shift; 
	my $xml_h = shift ; 
	foreach my $hn (keys %$xml_h) {
		if($xml_h->{$hn}{ip} eq $ip) {
			print STDERR __LINE__.": $ip: Already defined in DHCP Config\n"; 
			return 0; 
		}
	}

	if(! is_private_ipv4($ip)) {
		print STDERR __LINE__.": $ip: Invalid IP\n"; 
		return 0; 
	}
	if( CheckPing($ip)) {
		print STDERR __LINE__.": $ip pings\n"; 
		return 0 ;
	} 
	if( CheckDNS($ip) ) {
		print STDERR __LINE__.": $ip resolves\n"; 
		return 0 ;
	}
	return 1; 

}

sub CheckPing {
	my $ip = shift;
	my $p = Net::Ping->new(); 
	if ($p->ping($ip))  {
		print STDERR __LINE__.": $ip is pingable\n"; 
		$p->close();
		return 1  ; 
	}
	$p->close();
	return 0  ; 
}

sub VerifyValidHost {
	my $h = shift; 
	my $xmlh = shift; 
	my $resolver = new Net::DNS::Resolver();
	my $packet = $resolver->query($h) ; 
	if(defined $xmlh->{$h}) {
		print STDERR __LINE__.": ERROR: $h is already defined in DHCP config\n"; 
		return 0 ; 
	}
	if(defined $packet) {
		print STDERR __LINE__.": $h is already defined in DNS\n"; 
		return 0 ; 
	}
	return 1; 
}
sub CheckDNS {
	my $ip = shift;
	my $resolver = new Net::DNS::Resolver();
	my $packet = $resolver->query($ip) ; 
	if(defined $packet) {
		print STDERR __LINE__.": $ip is already defined in DNS\n"; 
		return 1 ; 
	}
	return 0; 
}
